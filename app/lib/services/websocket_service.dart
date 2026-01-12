import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/message.dart';

enum ConnectionStatus { disconnected, connecting, connected }

class WebSocketService {
  static const String wsUrl = 'ws://localhost:8080/ws';

  WebSocketChannel? _channel;
  ConnectionStatus _status = ConnectionStatus.disconnected;
  Timer? _reconnectTimer;
  Timer? _pingTimer;

  String? _token;

  final _messageController = StreamController<Message>.broadcast();
  final _typingController = StreamController<Map<String, dynamic>>.broadcast();
  final _presenceController = StreamController<Map<String, dynamic>>.broadcast();
  final _ackController = StreamController<Map<String, dynamic>>.broadcast();
  final _statusController = StreamController<ConnectionStatus>.broadcast();
  final _messageEditedController = StreamController<Map<String, dynamic>>.broadcast();
  final _messageDeletedController = StreamController<Map<String, dynamic>>.broadcast();

  Stream<Message> get messageStream => _messageController.stream;
  Stream<Map<String, dynamic>> get typingStream => _typingController.stream;
  Stream<Map<String, dynamic>> get presenceStream => _presenceController.stream;
  Stream<Map<String, dynamic>> get ackStream => _ackController.stream;
  Stream<ConnectionStatus> get statusStream => _statusController.stream;
  Stream<Map<String, dynamic>> get messageEditedStream => _messageEditedController.stream;
  Stream<Map<String, dynamic>> get messageDeletedStream => _messageDeletedController.stream;

  ConnectionStatus get status => _status;
  bool get isConnected => _status == ConnectionStatus.connected;

  void connect(String token) {
    _token = token;
    _connect();
  }

  void _connect() {
    if (_status == ConnectionStatus.connecting) return;

    _updateStatus(ConnectionStatus.connecting);

    try {
      final uri = Uri.parse('$wsUrl?token=$_token');
      _channel = WebSocketChannel.connect(uri);

      _channel!.stream.listen(
        _handleMessage,
        onDone: _handleDisconnect,
        onError: (error) {
          print('WebSocket error: $error');
          _handleDisconnect();
        },
      );

      _updateStatus(ConnectionStatus.connected);
      _startPingTimer();

    } catch (e) {
      print('Failed to connect: $e');
      _handleDisconnect();
    }
  }

  void _handleMessage(dynamic data) {
    try {
      final json = jsonDecode(data as String) as Map<String, dynamic>;
      final type = json['type'] as String?;

      switch (type) {
        case 'message':
          _messageController.add(Message.fromJson(json));
          break;
        case 'typing':
          _typingController.add(json);
          break;
        case 'presence':
          _presenceController.add(json);
          break;
        case 'ack':
          _ackController.add(json);
          break;
        case 'message_edited':
          _messageEditedController.add(json);
          break;
        case 'message_deleted':
          _messageDeletedController.add(json);
          break;
        case 'error':
          print('Server error: ${json['error']}');
          break;
      }
    } catch (e) {
      print('Failed to parse message: $e');
    }
  }

  void _handleDisconnect() {
    _updateStatus(ConnectionStatus.disconnected);
    _stopPingTimer();
    _channel = null;

    // Schedule reconnect
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), () {
      if (_token != null) {
        _connect();
      }
    });
  }

  void _updateStatus(ConnectionStatus status) {
    _status = status;
    _statusController.add(status);
  }

  void _startPingTimer() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      if (isConnected) {
        // WebSocket ping is handled at protocol level
      }
    });
  }

  void _stopPingTimer() {
    _pingTimer?.cancel();
    _pingTimer = null;
  }

  void sendMessage({
    String? to,         // For DMs
    String? groupId,    // For group messages
    String? content,
    String? mediaId,
    String? replyToId,  // For replies
  }) {
    if (!isConnected) {
      print('Cannot send message: not connected');
      return;
    }

    if (to == null && groupId == null) {
      print('Cannot send message: no recipient or group specified');
      return;
    }

    final message = {
      'type': 'message',
      if (to != null) 'to': to,
      if (groupId != null) 'group_id': groupId,
      if (content != null) 'content': content,
      if (mediaId != null) 'media_id': mediaId,
      if (replyToId != null) 'reply_to_id': replyToId,
    };

    _channel?.sink.add(jsonEncode(message));
  }

  void sendTyping({required String to, required bool typing}) {
    if (!isConnected) return;

    final message = {
      'type': 'typing',
      'to': to,
      'typing': typing,
    };

    _channel?.sink.add(jsonEncode(message));
  }

  void sendAck({required String messageId, required String status}) {
    if (!isConnected) return;

    final message = {
      'type': 'ack',
      'message_id': messageId,
      'status': status,
    };

    _channel?.sink.add(jsonEncode(message));
  }

  void sendMessageEdit({required String messageId, required String content}) {
    if (!isConnected) return;

    final message = {
      'type': 'message_edit',
      'message_id': messageId,
      'content': content,
    };

    _channel?.sink.add(jsonEncode(message));
  }

  void sendMessageDelete({required String messageId, required String deleteFor}) {
    if (!isConnected) return;

    final message = {
      'type': 'message_delete',
      'message_id': messageId,
      'delete_for': deleteFor, // "me" or "everyone"
    };

    _channel?.sink.add(jsonEncode(message));
  }

  void disconnect() {
    _reconnectTimer?.cancel();
    _stopPingTimer();
    _channel?.sink.close();
    _channel = null;
    _token = null;
    _updateStatus(ConnectionStatus.disconnected);
  }

  void dispose() {
    disconnect();
    _messageController.close();
    _typingController.close();
    _presenceController.close();
    _ackController.close();
    _statusController.close();
    _messageEditedController.close();
    _messageDeletedController.close();
  }
}
