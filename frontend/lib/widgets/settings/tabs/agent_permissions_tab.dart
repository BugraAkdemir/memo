import 'package:flutter/material.dart';
import '../../agent/permission_history.dart';

class AgentPermissionsTab extends StatelessWidget {
  const AgentPermissionsTab({super.key});

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: PermissionHistory(),
    );
  }
}
