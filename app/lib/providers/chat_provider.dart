import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';
import '../models/message.dart';
import '../models/conversation.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';
import 'auth_provider.dart';

class ChatState {
  final Map<String, List<Message>> messages; // keyed by other user ID
  final List<Conversation> conversations;
  final Map<String, bool> typingStatus; // keyed by user ID
  final bool isLoading;
  final String? error;

  ChatState({
    this.messages = const {},
    this.conversations = const [],
    this.typingStatus = const {},
    this.isLoading = false,
    this.error,
  });

  ChatState copyWith({
    Map<String, List<Message>>? messages,
    List<Conversation>? conversations,
    Map<String, bool>? typingStatus,
    bool? isLoading,
    String? error,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      conversations: conversations ?? this.conversations,
      typingStatus: typingStatus ?? this.typingStatus,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class ChatNotifier extends StateNotifier<ChatState> {
  final ApiService _apiService;
  final WebSocketService _webSocketService;
  final String _currentUserId;
  final _uuid = const Uuid();

  StreamSubscription? _messageSubscription;
  StreamSubscription? _typingSubscription;
  StreamSubscription? _ackSubscription;
  StreamSubscription? _messageEditedSubscription;
  StreamSubscription? _messageDeletedSubscription;

  ChatNotifier(
    this._apiService,
    this._webSocketService,
    this._currentUserId,
  ) : super(ChatState()) {
    _setupListeners();
  }

  void _setupListeners() {
    _messageSubscription = _webSocketService.messageStream.listen(_handleIncomingMessage);
    _typingSubscription = _webSocketService.typingStream.listen(_handleTyping);
    _ackSubscription = _webSocketService.ackStream.listen(_handleAck);
    _messageEditedSubscription = _webSocketService.messageEditedStream.listen(_handleMessageEdited);
    _messageDeletedSubscription = _webSocketService.messageDeletedStream.listen(_handleMessageDeleted);
  }

  void _handleIncomingMessage(Message message) {
    final otherUserId = message.senderId == _currentUserId
        ? message.recipientId
        : message.senderId;

    final currentMessages = state.messages[otherUserId] ?? [];
    final updatedMessages = [message, ...currentMessages];

    state = state.copyWith(
      messages: {...state.messages, otherUserId: updatedMessages},
    );

    // Mark as read if we're currently viewing this chat
    _webSocketService.sendAck(messageId: message.id, status: 'read');
  }

  void _handleTyping(Map<String, dynamic> data) {
    final from = data['from'] as String?;
    final typing = data['typing'] as bool? ?? false;

    if (from != null) {
      state = state.copyWith(
        typingStatus: {...state.typingStatus, from: typing},
      );

      // Clear typing indicator after 3 seconds
      if (typing) {
        Future.delayed(const Duration(seconds: 3), () {
          if (state.typingStatus[from] == true) {
            state = state.copyWith(
              typingStatus: {...state.typingStatus, from: false},
            );
          }
        });
      }
    }
  }

  void _handleAck(Map<String, dynamic> data) {
    final messageId = data['message_id'] as String?;
    final status = data['status'] as String?;

    if (messageId != null && status != null) {
      final newMessages = <String, List<Message>>{};

      for (final entry in state.messages.entries) {
        newMessages[entry.key] = entry.value.map((msg) {
          if (msg.id == messageId) {
            return msg.copyWith(status: Message.parseStatus(status));
          }
          return msg;
        }).toList();
      }

      state = state.copyWith(messages: newMessages);
    }
  }

  void _handleMessageEdited(Map<String, dynamic> data) {
    final messageId = data['message_id'] as String?;
    final content = data['content'] as String?;
    final editedAtStr = data['edited_at'] as String?;

    if (messageId != null) {
      final editedAt = editedAtStr != null ? DateTime.parse(editedAtStr) : DateTime.now();
      final newMessages = <String, List<Message>>{};

      for (final entry in state.messages.entries) {
        newMessages[entry.key] = entry.value.map((msg) {
          if (msg.id == messageId) {
            return msg.copyWith(
              content: content ?? msg.content,
              editedAt: editedAt,
            );
          }
          return msg;
        }).toList();
      }

      state = state.copyWith(messages: newMessages);
    }
  }

  void _handleMessageDeleted(Map<String, dynamic> data) {
    final messageId = data['message_id'] as String?;

    if (messageId != null) {
      final newMessages = <String, List<Message>>{};

      for (final entry in state.messages.entries) {
        newMessages[entry.key] = entry.value.map((msg) {
          if (msg.id == messageId) {
            return msg.copyWith(isDeleted: true);
          }
          return msg;
        }).toList();
      }

      state = state.copyWith(messages: newMessages);
    }
  }

  Future<void> loadConversations() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final conversations = await _apiService.getConversations();
      state = state.copyWith(
        conversations: conversations,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to load conversations',
        isLoading: false,
      );
    }
  }

  Future<void> loadMessages(String userId) async {
    try {
      final messages = await _apiService.getMessages(userId);
      state = state.copyWith(
        messages: {...state.messages, userId: messages},
      );
    } catch (e) {
      state = state.copyWith(error: 'Failed to load messages');
    }
  }

  void sendMessage(String toUserId, {String? content, String? mediaId, String? replyToId}) {
    if (content == null && mediaId == null) return;

    // Find the message being replied to for the preview
    ReplyPreview? replyTo;
    if (replyToId != null) {
      final currentMessages = state.messages[toUserId] ?? [];
      final replyMessage = currentMessages.where((m) => m.id == replyToId).firstOrNull;
      if (replyMessage != null) {
        replyTo = ReplyPreview(
          id: replyMessage.id,
          senderId: replyMessage.senderId,
          content: replyMessage.content,
        );
      }
    }

    // Create optimistic message
    final message = Message(
      id: _uuid.v4(),
      senderId: _currentUserId,
      recipientId: toUserId,
      content: content,
      mediaId: mediaId,
      replyToId: replyToId,
      replyTo: replyTo,
      status: MessageStatus.sent,
      createdAt: DateTime.now(),
    );

    // Add to local state immediately
    final currentMessages = state.messages[toUserId] ?? [];
    state = state.copyWith(
      messages: {...state.messages, toUserId: [message, ...currentMessages]},
    );

    // Send via WebSocket
    _webSocketService.sendMessage(
      to: toUserId,
      content: content,
      mediaId: mediaId,
      replyToId: replyToId,
    );
  }

  void sendTyping(String toUserId, bool typing) {
    _webSocketService.sendTyping(to: toUserId, typing: typing);
  }

  bool isTyping(String userId) {
    return state.typingStatus[userId] ?? false;
  }

  void editMessage(String messageId, String newContent) {
    _webSocketService.sendMessageEdit(messageId: messageId, content: newContent);
  }

  void deleteMessage(String messageId, {bool forEveryone = false}) {
    _webSocketService.sendMessageDelete(
      messageId: messageId,
      deleteFor: forEveryone ? 'everyone' : 'me',
    );
  }

  @override
  void dispose() {
    _messageSubscription?.cancel();
    _typingSubscription?.cancel();
    _ackSubscription?.cancel();
    _messageEditedSubscription?.cancel();
    _messageDeletedSubscription?.cancel();
    super.dispose();
  }
}

final chatProvider = StateNotifierProvider<ChatNotifier, ChatState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  final webSocketService = ref.watch(webSocketServiceProvider);
  final authState = ref.watch(authProvider);

  return ChatNotifier(
    apiService,
    webSocketService,
    authState.user?.id ?? '',
  );
});
