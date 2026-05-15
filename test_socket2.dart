import 'dart:io';
void main() async {
  try {
    var socket = await Socket.connect('127.0.0.1', 8888);
  } catch (e) {
    print(e);
  }
}
