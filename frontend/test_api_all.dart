import 'package:dio/dio.dart';
import 'lib/core/api_client.dart';
void main() async {
  final client = MemoApiClient();
  
  try { print("isAlive: ${await client.isAlive()}"); } catch (e) { print("isAlive error: $e"); }
  try { print("listChats: ${await client.listChats()}"); } catch (e) { print("listChats error: $e"); }
  try { print("listLocalModels: ${await client.listLocalModels()}"); } catch (e) { print("listLocalModels error: $e"); }
  try { print("getStatus: ${await client.getStatus()}"); } catch (e) { print("getStatus error: $e"); }
}
