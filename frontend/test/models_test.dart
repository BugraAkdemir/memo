import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/models/whatsapp.dart';
import 'package:memo_flutter/models/token_usage.dart';
import 'package:memo_flutter/models/activity_step.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/orchestra_config.dart';

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

    test('isUser and isAssistant computed correctly', () {
      final userMsg = ChatMessage(role: 'user', content: 'hi', timestamp: '12:00');
      expect(userMsg.isUser, true);
      expect(userMsg.isAssistant, false);

      final assistMsg = ChatMessage(role: 'assistant', content: 'hello', timestamp: '12:00');
      expect(assistMsg.isAssistant, true);
      expect(assistMsg.isUser, false);
    });

    test('hasImage and hasFile computed correctly', () {
      final withImg = ChatMessage(role: 'user', content: 'look', imagePath: '/img.png', timestamp: '12:00');
      expect(withImg.hasImage, true);
      expect(withImg.hasFile, false);

      final withFile = ChatMessage(role: 'user', content: 'here', filePath: '/doc.pdf', timestamp: '12:00');
      expect(withFile.hasFile, true);
      expect(withFile.hasImage, false);
    });

    test('hasThinking computed correctly', () {
      final withThinking = ChatMessage(role: 'assistant', content: 'answer', thinking: 'thought', timestamp: '12:00');
      expect(withThinking.hasThinking, true);

      final without = ChatMessage(role: 'assistant', content: 'answer', timestamp: '12:00');
      expect(without.hasThinking, false);
    });
  });

  group('StreamChunk', () {
    test('fromJson parses all fields', () {
      final chunk = StreamChunk.fromJson({
        'content': 'Hello',
        'finish_reason': 'stop',
        'thinking': 'hmm',
      });
      expect(chunk.content, 'Hello');
      expect(chunk.finishReason, 'stop');
      expect(chunk.thinking, 'hmm');
    });

    test('fromJson handles null fields', () {
      final chunk = StreamChunk.fromJson({});
      expect(chunk.content, '');
      expect(chunk.finishReason, isNull);
      expect(chunk.thinking, isNull);
    });
  });

  group('ChatSession', () {
    test('fromJson parses all fields', () {
      final session = ChatSession.fromJson({
        'id': 'abc123',
        'title': 'Test Chat',
        'created_at': '2024-01-01T00:00:00',
        'updated_at': '2024-01-02T00:00:00',
        'msg_count': 42,
        'project_path': '/home/project',
      });
      expect(session.id, 'abc123');
      expect(session.title, 'Test Chat');
      expect(session.createdAt, '2024-01-01T00:00:00');
      expect(session.updatedAt, '2024-01-02T00:00:00');
      expect(session.msgCount, 42);
      expect(session.projectPath, '/home/project');
      expect(session.isAgentChat, true);
    });

    test('fromJson handles defaults', () {
      final session = ChatSession.fromJson({'id': 'xyz'});
      expect(session.id, 'xyz');
      expect(session.title, 'New Chat');
      expect(session.createdAt, '');
      expect(session.updatedAt, '');
      expect(session.msgCount, 0);
      expect(session.isAgentChat, false);
    });
  });

  group('AgentEvent', () {
    test('fromJson parses all fields', () {
      final event = AgentEvent.fromJson({
        'type': 'tool_executing',
        'request_id': 'req-1',
        'tool': 'search_files',
        'args': {'pattern': '*.dart'},
        'result': '3 files found',
        'error': null,
        'danger_level': 'safe',
        'duration_ms': 1500,
        'content': 'searching...',
        'preview': '*.dart',
      });
      expect(event.type, 'tool_executing');
      expect(event.requestId, 'req-1');
      expect(event.toolName, 'search_files');
      expect(event.args, {'pattern': '*.dart'});
      expect(event.result, '3 files found');
      expect(event.error, isNull);
      expect(event.dangerLevel, 'safe');
      expect(event.durationMs, 1500);
      expect(event.content, 'searching...');
      expect(event.preview, '*.dart');
    });

    test('fromJson handles minimal payload', () {
      final event = AgentEvent.fromJson({'type': 'tool_result'});
      expect(event.type, 'tool_result');
      expect(event.requestId, isNull);
      expect(event.toolName, isNull);
      expect(event.durationMs, isNull);
    });

    test('fromJson defaults type to unknown', () {
      final event = AgentEvent.fromJson({});
      expect(event.type, 'unknown');
    });

    test('toJson round-trips', () {
      final original = AgentEvent(
        type: 'tool_error',
        requestId: 'req-2',
        toolName: 'run_command',
        args: {'cmd': 'ls'},
        result: null,
        error: 'permission denied',
        dangerLevel: 'medium',
        durationMs: 500,
      );
      final json = original.toJson();
      final restored = AgentEvent.fromJson(json);
      expect(restored.type, original.type);
      expect(restored.requestId, original.requestId);
      expect(restored.toolName, original.toolName);
      expect(restored.error, original.error);
      expect(restored.durationMs, original.durationMs);
    });

    test('toJson omits null fields', () {
      final event = AgentEvent(type: 'final_response');
      final json = event.toJson();
      expect(json.containsKey('request_id'), false);
      expect(json.containsKey('tool'), false);
      expect(json.containsKey('error'), false);
    });
  });

  group('AgentPermission', () {
    test('fromJson parses all fields', () {
      final perm = AgentPermission.fromJson({
        'id': 'perm-1',
        'tool_name': 'run_command',
        'args_hash': 'a1b2c3',
        'policy': 'allow',
        'created_at': '2024-01-01',
        'updated_at': '2024-01-02',
      });
      expect(perm.id, 'perm-1');
      expect(perm.toolName, 'run_command');
      expect(perm.argsHash, 'a1b2c3');
      expect(perm.policy, 'allow');
      expect(perm.createdAt, '2024-01-01');
      expect(perm.updatedAt, '2024-01-02');
    });

    test('fromJson handles null fields', () {
      final perm = AgentPermission.fromJson({});
      expect(perm.id, '');
      expect(perm.toolName, '');
      expect(perm.policy, '');
    });

    test('toJson round-trips', () {
      final original = AgentPermission(
        id: 'perm-2',
        toolName: 'file_write',
        argsHash: 'd4e5f6',
        policy: 'deny',
        createdAt: '2024-01-01',
        updatedAt: '2024-01-02',
      );
      final json = original.toJson();
      final restored = AgentPermission.fromJson(json);
      expect(restored.id, original.id);
      expect(restored.toolName, original.toolName);
      expect(restored.policy, original.policy);
    });
  });

  group('WhatsAppMessage', () {
    test('fromJson parses all fields', () {
      final msg = WhatsAppMessage.fromJson({
        'id': 'wa-1',
        'chat_jid': '123@s.whatsapp.net',
        'sender_jid': '456@s.whatsapp.net',
        'sender_name': 'Alice',
        'text': 'Hello!',
        'timestamp': '2024-01-01T12:00:00.000Z',
        'from_me': false,
      });
      expect(msg.id, 'wa-1');
      expect(msg.chatJid, '123@s.whatsapp.net');
      expect(msg.senderName, 'Alice');
      expect(msg.text, 'Hello!');
      expect(msg.fromMe, false);
      expect(msg.timestamp.year, 2024);
    });

    test('fromJson handles null fields', () {
      final msg = WhatsAppMessage.fromJson({});
      expect(msg.id, '');
      expect(msg.text, '');
      expect(msg.fromMe, false);
    });

    test('fromJson with null timestamp falls back to now', () {
      final msg = WhatsAppMessage.fromJson({'id': 'wa-2'});
      expect(msg.timestamp, isA<DateTime>());
    });
  });

  group('WhatsAppChatSummary', () {
    test('fromJson parses all fields', () {
      final summary = WhatsAppChatSummary.fromJson({
        'jid': '123@s.whatsapp.net',
        'display_name': 'Alice',
        'last_message': 'Hi!',
        'last_time': '2024-01-01T12:00:00.000Z',
        'unread': 5,
      });
      expect(summary.jid, '123@s.whatsapp.net');
      expect(summary.displayName, 'Alice');
      expect(summary.lastMessage, 'Hi!');
      expect(summary.unread, 5);
    });

    test('fromJson handles null fields', () {
      final summary = WhatsAppChatSummary.fromJson({});
      expect(summary.jid, '');
      expect(summary.unread, 0);
    });
  });

  group('WhatsAppStatus', () {
    test('fromJson parses all fields', () {
      final status = WhatsAppStatus.fromJson({
        'initialized': true,
        'connected': true,
        'logged_in': true,
        'reconnecting': false,
        'last_error': '',
        'qr_codes': ['qr1', 'qr2'],
      });
      expect(status.initialized, true);
      expect(status.connected, true);
      expect(status.loggedIn, true);
      expect(status.reconnecting, false);
      expect(status.qrCodes, ['qr1', 'qr2']);
    });

    test('fromJson handles null qr_codes', () {
      final status = WhatsAppStatus.fromJson({'qr_codes': null});
      expect(status.qrCodes, isEmpty);
    });

    test('fromJson handles non-list qr_codes', () {
      final status = WhatsAppStatus.fromJson({'qr_codes': 'not_a_list'});
      expect(status.qrCodes, isEmpty);
    });
  });

  group('TokenUsage', () {
    test('fromJson parses all fields', () {
      final usage = TokenUsage.fromJson({
        'input': 100,
        'output': 50,
        'budget': 128000,
      });
      expect(usage.input, 100);
      expect(usage.output, 50);
      expect(usage.total, 150);
      expect(usage.budget, 128000);
    });

    test('fromJson handles null fields', () {
      final usage = TokenUsage.fromJson({});
      expect(usage.input, 0);
      expect(usage.output, 0);
      expect(usage.total, 0);
    });

    test('fraction returns null when budget is zero', () {
      final usage = TokenUsage.fromJson({'input': 10, 'output': 5, 'budget': 0});
      expect(usage.fraction, isNull);
    });

    test('fraction clamps between 0 and 1', () {
      final usage = TokenUsage.fromJson({'input': 999999, 'output': 999999, 'budget': 100});
      expect(usage.fraction, 1.0);
    });

    test('fraction returns correct ratio', () {
      final usage = TokenUsage.fromJson({'input': 25, 'output': 25, 'budget': 100});
      expect(usage.fraction, 0.5);
    });
  });

  group('ActivityStep', () {
    test('fromActivityJson parses all fields', () {
      final step = ActivityStep.fromActivityJson({
        'id': 'step-1',
        'kind': 'tool',
        'title': 'Searching files',
        'subtitle': '*.dart',
        'status': 'done',
        'duration_ms': 1200,
        'detail': '3 files matched',
      });
      expect(step.id, 'step-1');
      expect(step.kind, 'tool');
      expect(step.title, 'Searching files');
      expect(step.subtitle, '*.dart');
      expect(step.status, StepStatus.done);
      expect(step.durationMs, 1200);
      expect(step.detail, '3 files matched');
    });

    test('fromActivityJson handles null fields', () {
      final step = ActivityStep.fromActivityJson({});
      expect(step.status, StepStatus.pending);
      expect(step.durationMs, 0);
      expect(step.detail, isNull);
    });

    test('statusFromString maps correctly', () {
      expect(ActivityStep.statusFromString('running'), StepStatus.running);
      expect(ActivityStep.statusFromString('done'), StepStatus.done);
      expect(ActivityStep.statusFromString('error'), StepStatus.error);
      expect(ActivityStep.statusFromString('unknown'), StepStatus.pending);
      expect(ActivityStep.statusFromString(null), StepStatus.pending);
    });

    test('copyWith overrides specified fields', () {
      final step = ActivityStep(id: 's1', kind: 'task', title: 'Plan');
      final copy = step.copyWith(status: StepStatus.running, durationMs: 500);
      expect(copy.id, 's1');
      expect(copy.status, StepStatus.running);
      expect(copy.durationMs, 500);
      expect(copy.title, 'Plan');
    });

    test('copyWith preserves omitted fields', () {
      final step = ActivityStep(id: 's2', kind: 'tool', title: 'Execute', subtitle: 'cmd', durationMs: 100);
      final copy = step.copyWith(status: StepStatus.done);
      expect(copy.title, 'Execute');
      expect(copy.subtitle, 'cmd');
      expect(copy.durationMs, 100);
    });
  });

  group('GPUInfo', () {
    test('fromJson parses all fields', () {
      final gpu = GPUInfo.fromJson({
        'type': 'nvidia',
        'name': 'RTX 4090',
        'vram_mb': 24576,
        'recommended_layers': 80,
        'description': 'NVIDIA Ada Lovelace',
        'ram_total_mb': 65536,
      });
      expect(gpu.type, 'nvidia');
      expect(gpu.name, 'RTX 4090');
      expect(gpu.vramMb, 24576);
      expect(gpu.recommendedLayers, 80);
      expect(gpu.hasGpu, true);
      expect(gpu.isNvidia, true);
      expect(gpu.isAmd, false);
    });

    test('fromJson defaults to CPU', () {
      final gpu = GPUInfo.fromJson({});
      expect(gpu.type, 'cpu');
      expect(gpu.hasGpu, false);
    });

    test('vramFormatted displays GB', () {
      final gpu = GPUInfo.fromJson({'vram_mb': 24576});
      expect(gpu.vramFormatted, contains('GB'));
    });

    test('vramFormatted displays MB under 1024', () {
      final gpu = GPUInfo.fromJson({'vram_mb': 512});
      expect(gpu.vramFormatted, '512 MB');
    });
  });

  group('ServerStatus', () {
    test('fromJson parses all fields', () {
      final status = ServerStatus.fromJson({
        'running': true,
        'model_path': '/models/llama.gguf',
        'model_name': 'llama3',
        'port': 8080,
        'pid': 12345,
        'gpu': {'type': 'nvidia', 'name': 'RTX 4090', 'vram_mb': 24576, 'recommended_layers': 80, 'description': '', 'ram_total_mb': 0},
      });
      expect(status.running, true);
      expect(status.modelPath, '/models/llama.gguf');
      expect(status.modelName, 'llama3');
      expect(status.port, 8080);
      expect(status.pid, 12345);
      expect(status.gpu.isNvidia, true);
    });

    test('fromJson handles missing gpu', () {
      final status = ServerStatus.fromJson({'running': false});
      expect(status.running, false);
      expect(status.gpu.type, 'cpu');
    });
  });

  group('MemoryFileInfo', () {
    test('fromJson parses all fields', () {
      final info = MemoryFileInfo.fromJson({
        'path': '/data/memory/mem1.gob',
        'name': 'mem1.gob',
        'size_kb': 1024,
        'modified': '2024-01-01',
      });
      expect(info.path, '/data/memory/mem1.gob');
      expect(info.name, 'mem1.gob');
      expect(info.sizeKb, 1024);
      expect(info.modified, '2024-01-01');
    });
  });

  group('MemorySearchResult', () {
    test('fromJson parses all fields', () {
      final result = MemorySearchResult.fromJson({
        'id': 'mem-1',
        'content': 'Some memory content',
        'similarity': 0.85,
        'timestamp': '2024-01-01',
        'user_msg': 'What is this?',
        'assist_msg': 'This is that',
        'match_type': 'vec',
        'importance': 5,
        'source': 'conversation',
        'tags': 'important',
        'retrieve_count': 10,
      });
      expect(result.id, 'mem-1');
      expect(result.similarity, 0.85);
      expect(result.importance, 5);
      expect(result.retrieveCount, 10);
    });
  });

  group('MemoryStats', () {
    test('fromJson parses all fields', () {
      final stats = MemoryStats.fromJson({
        'count': 100,
        'explicit_count': 10,
        'added_this_week': 5,
        'pending_deletion': 2,
        'top_retrieved': [
          {'id': 'mem-1', 'content': 'test', 'similarity': 0.9, 'timestamp': '', 'user_msg': '', 'assist_msg': '', 'match_type': 'vec'},
        ],
      });
      expect(stats.count, 100);
      expect(stats.explicitCount, 10);
      expect(stats.topRetrieved.length, 1);
      expect(stats.topRetrieved.first.id, 'mem-1');
    });

    test('fromJson handles missing top_retrieved', () {
      final stats = MemoryStats.fromJson({'count': 0});
      expect(stats.topRetrieved, isEmpty);
    });
  });

  group('LocalModel', () {
    test('fromJson parses all fields', () {
      final model = LocalModel.fromJson({
        'repo_id': 'meta/llama3',
        'filename': 'llama3-8b.gguf',
        'size': 8000000000,
        'path': '/models/llama3-8b.gguf',
        'is_embedding': false,
        'mmproj_path': 'mmproj-model.gguf',
        'supports_tools': true,
        'supports_vision': false,
        'supports_code': true,
        'tags': ['chat', 'instruct'],
      });
      expect(model.repoId, 'meta/llama3');
      expect(model.filename, 'llama3-8b.gguf');
      expect(model.size, 8000000000);
      expect(model.path, '/models/llama3-8b.gguf');
      expect(model.isEmbedding, false);
      expect(model.isVision, true);
      expect(model.supportsTools, true);
      expect(model.tags, ['chat', 'instruct']);
    });

    test('likelySupportsTools from metadata', () {
      final model = LocalModel(
        repoId: 'test/model',
        filename: 'model.gguf',
        size: 1000,
        path: '/m.gguf',
        isEmbedding: false,
        supportsTools: true,
      );
      expect(model.likelySupportsTools, true);
    });

    test('likelySupportsTools from heuristic', () {
      final model = LocalModel(
        repoId: 'meta/llama3',
        filename: 'llama3-8b-instruct.gguf',
        size: 1000,
        path: '/m.gguf',
        isEmbedding: false,
      );
      expect(model.likelySupportsTools, true);
    });

    test('likelySupportsTools false for embedding', () {
      final model = LocalModel(
        repoId: 'test/model',
        filename: 'model.gguf',
        size: 1000,
        path: '/m.gguf',
        isEmbedding: true,
      );
      expect(model.likelySupportsTools, false);
    });

    test('sizeFormatted formats GB', () {
      final model = LocalModel(
        repoId: 'test/m', filename: 'm.gguf', size: 5000000000, path: '/m.gguf', isEmbedding: false,
      );
      expect(model.sizeFormatted, contains('GB'));
    });

    test('sizeFormatted formats MB', () {
      final model = LocalModel(
        repoId: 'test/m', filename: 'm.gguf', size: 5000000, path: '/m.gguf', isEmbedding: false,
      );
      expect(model.sizeFormatted, contains('MB'));
    });
  });

  group('HFModelResult', () {
    test('fromJson parses all fields', () {
      final result = HFModelResult.fromJson({
        'id': 'meta/llama3',
        'author': 'meta',
        'downloads': 1000000,
        'likes': 5000,
        'tags': ['text-generation', 'function-calling', 'code'],
        'lastModified': '2024-01-01',
      });
      expect(result.id, 'meta/llama3');
      expect(result.author, 'meta');
      expect(result.downloads, 1000000);
      expect(result.likes, 5000);
      expect(result.tags, contains('function-calling'));
      expect(result.supportsTools, true);
      expect(result.supportsCode, true);
      expect(result.supportsVision, false);
    });
  });

  group('GGUFFile', () {
    test('fromJson parses fields', () {
      final file = GGUFFile.fromJson({'filename': 'q4_0.gguf', 'size': 4000000000});
      expect(file.filename, 'q4_0.gguf');
      expect(file.size, 4000000000);
    });

    test('sizeFormatted formats correctly', () {
      final file = GGUFFile.fromJson({'filename': 'm.gguf', 'size': 4000000000});
      expect(file.sizeFormatted, contains('GB'));
    });
  });

  group('DownloadProgress', () {
    test('fromJson parses all fields', () {
      final prog = DownloadProgress.fromJson({
        'active': true,
        'repo_id': 'meta/llama3',
        'filename': 'q4_0.gguf',
        'total_bytes': 4000000000,
        'downloaded': 2000000000,
        'percent': 50.0,
        'speed': '50 MB/s',
      });
      expect(prog.active, true);
      expect(prog.repoId, 'meta/llama3');
      expect(prog.percent, 50.0);
      expect(prog.speed, '50 MB/s');
    });

    test('fromJson defaults to inactive', () {
      final prog = DownloadProgress.fromJson({});
      expect(prog.active, false);
      expect(prog.percent, 0);
    });
  });

  group('ProviderConfig', () {
    test('fromJson parses all fields', () {
      final config = ProviderConfig.fromJson({
        'type': 'openai',
        'name': 'My OpenAI',
        'api_key': 'sk-xxx',
        'base_url': 'https://api.openai.com/v1',
        'model': 'gpt-4o',
        'enabled': true,
        'priority': 1,
        'temperature': 0.5,
        'top_p': 0.8,
        'max_tokens': 4096,
        'context_tokens': 128000,
        'connected': true,
        'error': null,
      });
      expect(config.type, 'openai');
      expect(config.name, 'My OpenAI');
      expect(config.apiKey, 'sk-xxx');
      expect(config.model, 'gpt-4o');
      expect(config.enabled, true);
      expect(config.priority, 1);
      expect(config.temperature, 0.5);
      expect(config.connected, true);
    });

    test('fromJson handles null fields', () {
      final config = ProviderConfig.fromJson({});
      expect(config.type, '');
      expect(config.name, '');
      expect(config.enabled, false);
      expect(config.priority, 0);
      expect(config.temperature, 0.7);
    });

    test('toJson round-trips', () {
      final original = ProviderConfig(
        type: 'claude',
        name: 'My Claude',
        apiKey: 'sk-ant-xxx',
        model: 'claude-sonnet-4-20250514',
        enabled: true,
        priority: 2,
        temperature: 0.3,
      );
      final json = original.toJson();
      final restored = ProviderConfig.fromJson(json);
      expect(restored.type, original.type);
      expect(restored.name, original.name);
      expect(restored.priority, original.priority);
    });

    test('copyWith overrides selected fields', () {
      final original = ProviderConfig(type: 'openai', name: 'Original', model: 'gpt-4');
      final copy = original.copyWith(name: 'Updated', temperature: 0.1);
      expect(copy.name, 'Updated');
      expect(copy.temperature, 0.1);
      expect(copy.type, 'openai');
    });
  });

  group('ProviderDefaults', () {
    test('defaultModels contains all providers', () {
      expect(ProviderDefaults.defaultModels, containsPair('openai', 'gpt-4o'));
      expect(ProviderDefaults.defaultModels, containsPair('ollama', 'llama3'));
    });

    test('needsApiKey returns correct values', () {
      expect(ProviderDefaults.needsApiKey('openai'), true);
      expect(ProviderDefaults.needsApiKey('ollama'), false);
      expect(ProviderDefaults.needsApiKey('custom'), false);
    });

    test('needsBaseUrl returns true only for custom', () {
      expect(ProviderDefaults.needsBaseUrl('custom'), true);
      expect(ProviderDefaults.needsBaseUrl('openai'), false);
    });
  });

  group('OrchestraConfig', () {
    test('fromJson parses all fields', () {
      final config = OrchestraConfig.fromJson({
        'enabled': true,
        'chief_type': 'claude',
        'chief_model': 'claude-sonnet-4-20250514',
        'roles': [
          {'role': 'planner', 'enabled': true, 'model_type': 'claude', 'model_name': 'claude-sonnet-4-20250514', 'system_prompt': 'Plan'},
        ],
      });
      expect(config.enabled, true);
      expect(config.chiefType, 'claude');
      expect(config.roles.length, 1);
      expect(config.roles.first.role, 'planner');
    });

    test('toJson round-trips', () {
      final original = OrchestraConfig(
        enabled: true,
        chiefType: 'openai',
        chiefModel: 'gpt-4o',
        roles: [RoleConfig(role: 'coder', enabled: true, modelType: 'openai', modelName: 'gpt-4o')],
      );
      final json = original.toJson();
      final restored = OrchestraConfig.fromJson(json);
      expect(restored.enabled, original.enabled);
      expect(restored.roles.length, original.roles.length);
    });

    test('copyWith overrides selected fields', () {
      final original = OrchestraConfig(enabled: false);
      final copy = original.copyWith(enabled: true);
      expect(copy.enabled, true);
    });
  });

  group('RoleConfig', () {
    test('fromJson parses all fields', () {
      final role = RoleConfig.fromJson({
        'role': 'planner',
        'enabled': true,
        'model_type': 'claude',
        'model_name': 'claude-sonnet-4-20250514',
        'system_prompt': 'You are a planner.',
      });
      expect(role.role, 'planner');
      expect(role.enabled, true);
      expect(role.modelType, 'claude');
      expect(role.systemPrompt, 'You are a planner.');
    });

    test('toJson round-trips', () {
      final original = RoleConfig(role: 'coder', enabled: true, modelType: 'openai', modelName: 'gpt-4o');
      final json = original.toJson();
      final restored = RoleConfig.fromJson(json);
      expect(restored.role, original.role);
      expect(restored.modelName, original.modelName);
    });

    test('copyWith overrides fields', () {
      final original = RoleConfig(role: 'dev', enabled: false);
      final copy = original.copyWith(enabled: true, modelName: 'gpt-4');
      expect(copy.enabled, true);
      expect(copy.modelName, 'gpt-4');
      expect(copy.role, 'dev');
    });
  });

  group('OrchestraDefaults', () {
    test('iconForRole returns icon for each role', () {
      expect(OrchestraDefaults.iconForRole('planner'), isNotEmpty);
      expect(OrchestraDefaults.iconForRole('frontend'), isNotEmpty);
      expect(OrchestraDefaults.iconForRole('unknown'), isNotEmpty);
    });

    test('labelForRole returns label for known roles', () {
      expect(OrchestraDefaults.labelForRole('planner'), 'Planner');
      expect(OrchestraDefaults.labelForRole('unknown'), 'unknown');
    });
  });
}
