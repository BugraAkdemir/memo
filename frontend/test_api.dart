// ignore_for_file: avoid_print
import 'lib/core/api_client.dart';
void main() async {
  final client = MemoApiClient(baseUrl: 'http://127.0.0.1:8090');
  try {
    print("Base URL is: ${client.baseUrl}");
    final res = await client.getVersion();
    print("Version: $res");
  } catch (e) {
    print("Error: $e");
  }
}
