import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/vad_assets.dart';

void main() {
  test('native VAD model path is a Flutter asset key', () {
    expect(vadModelBaseAssetPath, 'assets/vad/');
  });
}
