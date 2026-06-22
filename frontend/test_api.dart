// ignore_for_file: avoid_print
import 'lib/core/api_client.dart';
void main() async {
  final client = MemoApiClient();
  try {
    print("Base URL is: ${client.baseUrl}");
    final res = await client.getVersion();
    print("Version: $res");
  } catch (e) {
    print("Error: $e");
  }
}
