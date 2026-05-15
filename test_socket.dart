import 'dart:io';
void main() async {
  try {
    var socket = await Socket.connect('localhost', 12345);
  } catch (e) {
    print(e);
  }
}
