// ignore_for_file: avoid_print
import 'lib/core/api_client.dart';
void main() async {
  final client = MemoApiClient();
  
  try { print("isAlive: ${await client.isAlive()}"); } catch (e) { print("isAlive error: $e"); }
  try { print("listLocalModels: ${await client.listLocalModels()}"); } catch (e) { print("listLocalModels error: $e"); }
}
