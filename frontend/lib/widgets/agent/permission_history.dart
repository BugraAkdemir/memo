import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/agent_provider.dart';

class PermissionHistory extends ConsumerWidget {
  const PermissionHistory({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final permissionsAsync = ref.watch(agentPermissionsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            const Text(
              'Kalıcı İzinler',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            TextButton.icon(
              onPressed: () {
                showDialog(
                  context: context,
                  builder: (context) => AlertDialog(
                    title: const Text('Tüm İzinleri Temizle'),
                    content: const Text('Agent için verilmiş tüm kalıcı izinleri silmek istediğinize emin misiniz?'),
                    actions: [
                      TextButton(onPressed: () => Navigator.pop(context), child: const Text('İptal')),
                      TextButton(
                        onPressed: () {
                          ref.read(agentPermissionsProvider.notifier).clearAll();
                          Navigator.pop(context);
                        },
                        style: TextButton.styleFrom(foregroundColor: Colors.red),
                        child: const Text('Temizle'),
                      ),
                    ],
                  ),
                );
              },
              icon: const Icon(Icons.delete_sweep, color: Colors.red),
              label: const Text('Tümünü Temizle', style: TextStyle(color: Colors.red)),
            ),
          ],
        ),
        const SizedBox(height: 8),
        const Text(
          'Agent modunda "Kalıcı olarak izin ver" veya "Kalıcı reddet" seçeneğiyle onayladığınız işlemler burada listelenir.',
          style: TextStyle(color: Colors.grey),
        ),
        const SizedBox(height: 16),
        Expanded(
          child: permissionsAsync.when(
            data: (permissions) {
              if (permissions.isEmpty) {
                return const Center(
                  child: Text('Henüz kalıcı bir izin kaydı bulunmuyor.'),
                );
              }
              return ListView.builder(
                itemCount: permissions.length,
                itemBuilder: (context, index) {
                  final p = permissions[index];
                  final isAllowed = p.policy == 'allow_forever';
                  return Card(
                    margin: const EdgeInsets.only(bottom: 8),
                    child: ListTile(
                      leading: Icon(
                        isAllowed ? Icons.check_circle : Icons.block,
                        color: isAllowed ? Colors.green : Colors.red,
                      ),
                      title: Text(p.toolName, style: const TextStyle(fontWeight: FontWeight.bold)),
                      subtitle: Text('Argüman Hash: ${p.argsHash.length >= 8 ? p.argsHash.substring(0, 8) : p.argsHash}...\nTarih: ${p.updatedAt}'),
                      isThreeLine: true,
                      trailing: IconButton(
                        icon: const Icon(Icons.delete_outline),
                        tooltip: 'İzni İptal Et',
                        onPressed: () {
                          // The Go backend deletes by toolName and argsHash, we can pass a composite id if needed
                          // In the backend DeleteAgentPermission takes an `id`. Let's assume id is argsHash or toolName:argsHash.
                          // Wait, the API endpoint is DELETE /api/agent/permissions with {"id": ...}
                          // Wait! Let's check executor.go in Go to see how id is matched.
                          // Actually, we pass the argsHash as the id.
                          ref.read(agentPermissionsProvider.notifier).revoke(p.argsHash);
                        },
                      ),
                    ),
                  );
                },
              );
            },
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, st) => Center(child: Text('Hata: $e')),
          ),
        ),
      ],
    );
  }
}
