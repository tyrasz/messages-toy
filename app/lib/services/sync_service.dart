import 'dart:async';
import 'package:drift/drift.dart';
import '../models/message.dart';
import '../models/conversation.dart';
import 'api_service.dart';
import 'offline_database.dart';
import 'websocket_service.dart';

/// Handles synchronization between local cache and server
class SyncService {
  final OfflineDatabase _db;
  final ApiService _api;
  final WebSocketService _ws;

  bool _isSyncing = false;
  final _syncController = StreamController<SyncStatus>.broadcast();

  Stream<SyncStatus> get syncStream => _syncController.stream;

  SyncService({
    required OfflineDatabase db,
    required ApiService api,
    required WebSocketService ws,
  })  : _db = db,
        _api = api,
        _ws = ws {
    // Listen for reconnection to trigger sync
    _ws.statusStream.listen((status) {
      if (status == ConnectionStatus.connected) {
        sync();
      }
    });
  }

  /// Full sync - called on app start and reconnection
  Future<void> sync() async {
    if (_isSyncing) return;
    _isSyncing = true;
    _syncController.add(SyncStatus.syncing);

    try {
      // 1. Send any pending messages
      await _sendPendingMessages();

      // 2. Fetch new messages since last sync
      await _fetchNewMessages();

      // 3. Update conversations list
      await _syncConversations();

      // 4. Update contacts
      await _syncContacts();

      await _db.setLastSyncTime(DateTime.now());
      _syncController.add(SyncStatus.completed);
    } catch (e) {
      print('Sync failed: $e');
      _syncController.add(SyncStatus.failed);
    } finally {
      _isSyncing = false;
    }
  }

  /// Send messages that were created while offline
  Future<void> _sendPendingMessages() async {
    final pending = await _db.getPendingMessages();

    for (final msg in pending) {
      try {
        // Send via WebSocket
        _ws.sendMessage(
          to: msg.recipientId,
          groupId: msg.groupId,
          content: msg.content,
          mediaId: msg.mediaId,
          replyToId: msg.replyToId,
        );

        // Remove from pending queue
        await _db.removePendingMessage(msg.id);
      } catch (e) {
        // Increment retry count
        await _db.incrementRetryCount(msg.id);

        // Remove if too many retries
        if (msg.retryCount >= 5) {
          await _db.removePendingMessage(msg.id);
          print('Dropped message after 5 retries: ${msg.tempId}');
        }
      }
    }
  }

  /// Fetch messages created since last sync
  Future<void> _fetchNewMessages() async {
    final lastSync = await _db.getLastSyncTime();

    // Fetch from API with since parameter
    final conversations = await _api.getConversations();

    for (final conv in conversations) {
      final messages = await _api.getMessages(
        conv.user?.id ?? conv.groupId ?? '',
        since: lastSync,
      );

      // Save to local database
      final cachedMessages = messages.map((m) => _messageToCached(m)).toList();
      await _db.saveMessages(cachedMessages);
    }
  }

  /// Sync conversations list
  Future<void> _syncConversations() async {
    final conversations = await _api.getConversations();

    for (final conv in conversations) {
      await _db.saveConversation(
        CachedConversation(
          oderId: conv.user?.id ?? conv.groupId ?? '',
          isGroup: conv.groupId != null,
          lastMessageId: conv.lastMessage?.id,
          lastMessageContent: conv.lastMessage?.content,
          lastMessageAt: conv.lastMessage?.createdAt,
          unreadCount: conv.unreadCount,
          isMuted: false,
          isArchived: false,
        ),
      );
    }
  }

  /// Sync contacts list
  Future<void> _syncContacts() async {
    final contacts = await _api.getContacts();

    final cachedContacts = contacts.map((c) => CachedContact(
      id: c.id,
      username: c.username,
      displayName: c.displayName,
      avatarUrl: c.avatarUrl,
      about: c.about,
      isBlocked: false,
      lastSeen: c.lastSeen,
      updatedAt: DateTime.now(),
    )).toList();

    await _db.saveContacts(cachedContacts);
  }

  /// Convert API message to cached message
  CachedMessage _messageToCached(Message m) {
    return CachedMessage(
      id: m.id,
      senderId: m.senderId,
      recipientId: m.recipientId,
      groupId: m.groupId,
      content: m.content,
      mediaId: m.mediaId,
      mediaUrl: m.mediaUrl,
      mediaType: m.mediaType,
      replyToId: m.replyToId,
      forwardedFrom: m.forwardedFrom,
      status: m.status,
      createdAt: m.createdAt,
      editedAt: m.editedAt,
      isDeleted: m.isDeleted,
      isSynced: true,
    );
  }

  /// Save an incoming message to local cache
  Future<void> cacheMessage(Message message) async {
    await _db.saveMessage(_messageToCached(message));

    // Update conversation
    final oderId = message.groupId ?? message.senderId;
    await _db.saveConversation(
      CachedConversation(
        oderId: oderId,
        isGroup: message.groupId != null,
        lastMessageId: message.id,
        lastMessageContent: message.content,
        lastMessageAt: message.createdAt,
        unreadCount: 0, // Will be updated separately
        isMuted: false,
        isArchived: false,
      ),
    );
  }

  /// Queue a message to be sent when online
  Future<void> queueMessage({
    String? recipientId,
    String? groupId,
    String? content,
    String? mediaId,
    String? replyToId,
  }) async {
    final tempId = DateTime.now().millisecondsSinceEpoch.toString();

    await _db.addPendingMessage(
      PendingMessagesCompanion(
        tempId: Value(tempId),
        recipientId: Value(recipientId),
        groupId: Value(groupId),
        content: Value(content),
        mediaId: Value(mediaId),
        replyToId: Value(replyToId),
        createdAt: Value(DateTime.now()),
      ),
    );

    // Also save to messages cache for immediate display
    await _db.saveMessage(
      CachedMessage(
        id: tempId,
        senderId: '', // Will be set by provider
        recipientId: recipientId,
        groupId: groupId,
        content: content,
        mediaId: mediaId,
        replyToId: replyToId,
        status: 'pending',
        createdAt: DateTime.now(),
        isSynced: false,
      ),
    );
  }

  /// Get cached messages for a conversation
  Future<List<CachedMessage>> getCachedMessages(String oderId, {int limit = 50}) async {
    return _db.getMessages(oderId, limit: limit);
  }

  /// Get cached conversations
  Future<List<CachedConversation>> getCachedConversations() async {
    return _db.getConversations();
  }

  /// Clear all cached data (for logout)
  Future<void> clearCache() async {
    await _db.clearAll();
  }

  void dispose() {
    _syncController.close();
  }
}

enum SyncStatus {
  idle,
  syncing,
  completed,
  failed,
}
