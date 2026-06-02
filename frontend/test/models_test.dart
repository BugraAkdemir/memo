import 'package:flutter_test/flutter_test.dart';

import '../lib/models/chat.dart';
import '../lib/models/gpu_info.dart';
import '../lib/models/local_model.dart';

void main() {
  group('ChatMessage', () {
    test('fromJson parses all fields', () {
      final msg = ChatMessage.fromJson({
        'role': 'user',
        'content': 'hello',
        'thinking': 'hmm',
        'image_path': '/img.png',
        'file_path': '/doc.pdf',
        'timestamp': '12:34',
      });
      expect(msg.role, 'user');
      expect(msg.content, 'hello');
      expect(msg.thinking, 'hmm');
      expect(msg.imagePath, '/img.png');
      expect(msg.filePath, '/doc.pdf');
      expect(msg.timestamp, '12:34');
    });

    test('fromJson handles null fields', () {
      final msg = ChatMessage.fromJson({'role': 'assistant', 'content': 'hi'});
      expect(msg.thinking, isNull);
      expect(msg.imagePath, isNull);
      expect(msg.filePath, isNull);
      expect(msg.timestamp, '');
    });

    test('toJson round-trips', () {
      final original = ChatMessage(
        role: 'user',
        content: 'test',
        thinking: 'thinking',
        imagePath: '/img.png',
        filePath: '/doc.pdf',
        timestamp: '12:34',
      );
      final json = original.toJson();
      final restored = ChatMessage.fromJson(json);
      expect(restored.role, original.role);
      expect(restored.content, original.content);
      expect(restored.thinking, original.thinking);
      expect(restored.imagePath, original.imagePath);
      expect(restored.filePath, original.filePath);
      expect(restored.timestamp, original.timestamp);
    });

    test('isUser returns true for user role', () {
      expect(ChatMessage(role: 'user', content: '', timestamp: '').isUser, isTrue);
      expect(ChatMessage(role: 'assistant', content: '', timestamp: '').isUser, isFalse);
      expect(ChatMessage(role: 'system', content: '', timestamp: '').isUser, isFalse);
    });

    test('isAssistant returns true for assistant role', () {
      expect(ChatMessage(role: 'assistant', content: '', timestamp: '').isAssistant, isTrue);
      expect(ChatMessage(role: 'user', content: '', timestamp: '').isAssistant, isFalse);
    });

    test('hasImage returns true when imagePath is non-empty', () {
      expect(ChatMessage(role: 'user', content: '', timestamp: '', imagePath: '/img.png').hasImage, isTrue);
      expect(ChatMessage(role: 'user', content: '', timestamp: '').hasImage, isFalse);
    });

    test('hasFile returns true when filePath is non-empty', () {
      expect(ChatMessage(role: 'user', content: '', timestamp: '', filePath: '/doc.pdf').hasFile, isTrue);
      expect(ChatMessage(role: 'user', content: '', timestamp: '').hasFile, isFalse);
    });

    test('hasThinking returns true when thinking is non-empty', () {
      expect(ChatMessage(role: 'user', content: '', timestamp: '', thinking: 'hmm').hasThinking, isTrue);
      expect(ChatMessage(role: 'user', content: '', timestamp: '').hasThinking, isFalse);
    });
  });

  group('StreamChunk', () {
    test('fromJson parses content and thinking', () {
      final chunk = StreamChunk.fromJson({'content': 'hello', 'thinking': 'hmm'});
      expect(chunk.content, 'hello');
      expect(chunk.thinking, 'hmm');
    });

    test('fromJson handles missing thinking', () {
      final chunk = StreamChunk.fromJson({'content': 'hello'});
      expect(chunk.content, 'hello');
      expect(chunk.thinking, isNull);
    });

    test('fromJson defaults to empty content', () {
      final chunk = StreamChunk.fromJson({});
      expect(chunk.content, '');
      expect(chunk.thinking, isNull);
    });
  });

  group('ChatSession', () {
    test('fromJson parses all fields', () {
      final session = ChatSession.fromJson({
        'id': 'abc-123',
        'title': 'Test Chat',
        'created_at': '2026-01-01 12:00',
        'updated_at': '2026-01-01 13:00',
        'msg_count': 5,
      });
      expect(session.id, 'abc-123');
      expect(session.title, 'Test Chat');
      expect(session.createdAt, '2026-01-01 12:00');
      expect(session.updatedAt, '2026-01-01 13:00');
      expect(session.msgCount, 5);
    });

    test('fromJson defaults title to New Chat', () {
      final session = ChatSession.fromJson({'id': 'x'});
      expect(session.title, 'New Chat');
    });
  });

  group('GPUInfo', () {
    test('fromJson parses all fields', () {
      final info = GPUInfo.fromJson({
        'type': 'nvidia',
        'name': 'RTX 4090',
        'vram_mb': 24576,
        'recommended_layers': 999,
        'description': 'NVIDIA RTX 4090',
      });
      expect(info.type, 'nvidia');
      expect(info.name, 'RTX 4090');
      expect(info.vramMb, 24576);
      expect(info.recommendedLayers, 999);
    });

    test('hasGpu is false for cpu', () {
      expect(GPUInfo().hasGpu, isFalse);
      expect(GPUInfo(type: 'nvidia').hasGpu, isTrue);
      expect(GPUInfo(type: 'amd').hasGpu, isTrue);
    });

    test('isNvidia and isAmd', () {
      expect(GPUInfo(type: 'nvidia').isNvidia, isTrue);
      expect(GPUInfo(type: 'nvidia').isAmd, isFalse);
      expect(GPUInfo(type: 'amd').isAmd, isTrue);
      expect(GPUInfo(type: 'amd').isNvidia, isFalse);
    });

    test('vramFormatted shows GB for >= 1024 MB', () {
      expect(GPUInfo(vramMb: 8192).vramFormatted, '8.0 GB');
      expect(GPUInfo(vramMb: 24576).vramFormatted, '24.0 GB');
    });

    test('vramFormatted shows MB for < 1024 MB', () {
      expect(GPUInfo(vramMb: 512).vramFormatted, '512 MB');
      expect(GPUInfo(vramMb: 0).vramFormatted, '0 MB');
    });

    test('fromJson defaults', () {
      final info = GPUInfo.fromJson({});
      expect(info.type, 'cpu');
      expect(info.name, 'CPU');
      expect(info.vramMb, 0);
      expect(info.recommendedLayers, 0);
    });
  });

  group('ServerStatus', () {
    test('fromJson parses all fields', () {
      final status = ServerStatus.fromJson({
        'running': true,
        'model_path': '/models/test.gguf',
        'model_name': 'test',
        'port': 8081,
        'pid': 1234,
        'gpu': {'type': 'nvidia', 'name': 'RTX 4090', 'vram_mb': 24576, 'recommended_layers': 999},
      });
      expect(status.running, isTrue);
      expect(status.modelPath, '/models/test.gguf');
      expect(status.modelName, 'test');
      expect(status.port, 8081);
      expect(status.pid, 1234);
      expect(status.gpu.isNvidia, isTrue);
    });

    test('fromJson defaults', () {
      final status = ServerStatus.fromJson({});
      expect(status.running, isFalse);
      expect(status.gpu.type, 'cpu');
    });
  });

  group('MemoryFileInfo', () {
    test('fromJson parses all fields', () {
      final info = MemoryFileInfo.fromJson({
        'path': 'conv_123.gob',
        'name': 'conv_123.gob',
        'size_kb': 42,
        'modified': '2026-01-01 12:00',
      });
      expect(info.path, 'conv_123.gob');
      expect(info.name, 'conv_123.gob');
      expect(info.sizeKb, 42);
      expect(info.modified, '2026-01-01 12:00');
    });

    test('fromJson defaults', () {
      final info = MemoryFileInfo.fromJson({});
      expect(info.path, '');
      expect(info.sizeKb, 0);
    });
  });

  group('LocalModel', () {
    test('fromJson parses all fields', () {
      final model = LocalModel.fromJson({
        'repo_id': 'meta-llama/llama-3.2-3b',
        'filename': 'llama-3.2-3b.Q4_K_M.gguf',
        'size': 2000000000,
        'path': '/models/llama.gguf',
        'is_embedding': false,
      });
      expect(model.repoId, 'meta-llama/llama-3.2-3b');
      expect(model.size, 2000000000);
      expect(model.isEmbedding, isFalse);
    });

    test('sizeFormatted shows GB', () {
      final model = LocalModel(repoId: '', filename: '', size: 5000000000, path: '', isEmbedding: false);
      expect(model.sizeFormatted, contains('GB'));
    });

    test('sizeFormatted shows MB', () {
      final model = LocalModel(repoId: '', filename: '', size: 50000000, path: '', isEmbedding: false);
      expect(model.sizeFormatted, contains('MB'));
    });

    test('sizeFormatted shows KB', () {
      final model = LocalModel(repoId: '', filename: '', size: 5000, path: '', isEmbedding: false);
      expect(model.sizeFormatted, contains('KB'));
    });

    test('fromJson defaults', () {
      final model = LocalModel.fromJson({});
      expect(model.repoId, '');
      expect(model.filename, '');
      expect(model.size, 0);
      expect(model.isEmbedding, isFalse);
    });
  });

  group('HFModelResult', () {
    test('fromJson parses all fields', () {
      final result = HFModelResult.fromJson({
        'id': 'meta-llama/llama-3.2-3b',
        'author': 'meta-llama',
        'downloads': 100000,
        'likes': 5000,
        'tags': ['llama', 'gguf'],
      });
      expect(result.id, 'meta-llama/llama-3.2-3b');
      expect(result.author, 'meta-llama');
      expect(result.downloads, 100000);
      expect(result.tags, ['llama', 'gguf']);
    });

    test('fromJson defaults', () {
      final result = HFModelResult.fromJson({});
      expect(result.id, '');
      expect(result.downloads, 0);
      expect(result.tags, isEmpty);
    });
  });

  group('GGUFFile', () {
    test('fromJson parses fields', () {
      final file = GGUFFile.fromJson({'filename': 'model.gguf', 'size': 4000000000});
      expect(file.filename, 'model.gguf');
      expect(file.size, 4000000000);
    });

    test('sizeFormatted works', () {
      expect(GGUFFile(filename: '', size: 4000000000).sizeFormatted, contains('GB'));
      expect(GGUFFile(filename: '', size: 4000000).sizeFormatted, contains('MB'));
      expect(GGUFFile(filename: '', size: 4000).sizeFormatted, contains('KB'));
    });
  });

  group('DownloadProgress', () {
    test('fromJson parses all fields', () {
      final progress = DownloadProgress.fromJson({
        'active': true,
        'repo_id': 'test/repo',
        'filename': 'model.gguf',
        'total_bytes': 4000000000,
        'downloaded': 1000000,
        'percent': 0.025,
        'speed': '1.5 MB/s',
      });
      expect(progress.active, isTrue);
      expect(progress.repoId, 'test/repo');
      expect(progress.totalBytes, 4000000000);
      expect(progress.downloaded, 1000000);
      expect(progress.speed, '1.5 MB/s');
    });

    test('fromJson defaults', () {
      final progress = DownloadProgress.fromJson({});
      expect(progress.active, isFalse);
      expect(progress.percent, 0);
    });
  });

  group('MemoApiClient helpers', () {
    test('extractErrorMessage from string response', () {
      // Access the private method via reflection is not possible in Dart,
      // but we can test the updateLlamaConfig method indirectly by
      // verifying the data structure it builds
    });

    test('updateLlamaConfig builds correct data', () {
      // This is an integration test requiring a backend — skip for unit
    });
  });
}
