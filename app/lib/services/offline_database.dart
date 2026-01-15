import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';

part 'offline_database.g.dart';

// Messages table
class CachedMessages extends Table {
  TextColumn get id => text()();
  TextColumn get senderId => text()();
  TextColumn get recipientId => text().nullable()();
  TextColumn get groupId => text().nullable()();
  TextColumn get content => text().nullable()();
  TextColumn get mediaId => text().nullable()();
  TextColumn get mediaUrl => text().nullable()();
  TextColumn get mediaType => text().nullable()();
  TextColumn get replyToId => text().nullable()();
  TextColumn get forwardedFrom => text().nullable()();
  TextColumn get status => text().withDefault(const Constant('sent'))();
  DateTimeColumn get createdAt => dateTime()();
  DateTimeColumn get editedAt => dateTime().nullable()();
  BoolColumn get isDeleted => boolean().withDefault(const Constant(false))();
  BoolColumn get isSynced => boolean().withDefault(const Constant(true))();

  @override
  Set<Column> get primaryKey => {id};
}

// Conversations table (for quick access to conversation list)
class CachedConversations extends Table {
  TextColumn get oderId => text()(); // oderId = recipientId or groupId
  BoolColumn get isGroup => boolean().withDefault(const Constant(false))();
  TextColumn get lastMessageId => text().nullable()();
  TextColumn get lastMessageContent => text().nullable()();
  DateTimeColumn get lastMessageAt => dateTime().nullable()();
  IntColumn get unreadCount => integer().withDefault(const Constant(0))();
  BoolColumn get isMuted => boolean().withDefault(const Constant(false))();
  BoolColumn get isArchived => boolean().withDefault(const Constant(false))();

  @override
  Set<Column> get primaryKey => {oderId};
}

// Contacts table
class CachedContacts extends Table {
  TextColumn get id => text()();
  TextColumn get username => text()();
  TextColumn get displayName => text().nullable()();
  TextColumn get avatarUrl => text().nullable()();
  TextColumn get about => text().nullable()();
  BoolColumn get isBlocked => boolean().withDefault(const Constant(false))();
  DateTimeColumn get lastSeen => dateTime().nullable()();
  DateTimeColumn get updatedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

// Pending messages (queued while offline)
class PendingMessages extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get tempId => text()(); // Client-generated ID
  TextColumn get recipientId => text().nullable()();
  TextColumn get groupId => text().nullable()();
  TextColumn get content => text().nullable()();
  TextColumn get mediaId => text().nullable()();
  TextColumn get replyToId => text().nullable()();
  DateTimeColumn get createdAt => dateTime()();
  IntColumn get retryCount => integer().withDefault(const Constant(0))();
}

// Sync state tracking
class SyncState extends Table {
  TextColumn get key => text()();
  TextColumn get value => text()();
  DateTimeColumn get updatedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {key};
}

@DriftDatabase(tables: [
  CachedMessages,
  CachedConversations,
  CachedContacts,
  PendingMessages,
  SyncState,
])
class OfflineDatabase extends _$OfflineDatabase {
  OfflineDatabase() : super(_openConnection());

  @override
  int get schemaVersion => 1;

  static QueryExecutor _openConnection() {
    return driftDatabase(name: 'messenger_offline');
  }

  // Message operations
  Future<void> saveMessage(CachedMessage message) async {
    await into(cachedMessages).insertOnConflictUpdate(message);
  }

  Future<void> saveMessages(List<CachedMessage> messages) async {
    await batch((batch) {
      batch.insertAllOnConflictUpdate(cachedMessages, messages);
    });
  }

  Future<List<CachedMessage>> getMessages(String oderId, {int limit = 50, int offset = 0}) async {
    final query = select(cachedMessages)
      ..where((m) => m.recipientId.equals(oderId) | m.groupId.equals(oderId))
      ..orderBy([(m) => OrderingTerm.desc(m.createdAt)])
      ..limit(limit, offset: offset);
    return query.get();
  }

  Future<CachedMessage?> getMessage(String id) async {
    final query = select(cachedMessages)..where((m) => m.id.equals(id));
    return query.getSingleOrNull();
  }

  Future<void> markMessageSynced(String id) async {
    await (update(cachedMessages)..where((m) => m.id.equals(id)))
        .write(const CachedMessagesCompanion(isSynced: Value(true)));
  }

  Future<List<CachedMessage>> getUnsyncedMessages() async {
    final query = select(cachedMessages)
      ..where((m) => m.isSynced.equals(false))
      ..orderBy([(m) => OrderingTerm.asc(m.createdAt)]);
    return query.get();
  }

  // Conversation operations
  Future<void> saveConversation(CachedConversation conversation) async {
    await into(cachedConversations).insertOnConflictUpdate(conversation);
  }

  Future<List<CachedConversation>> getConversations({bool includeArchived = false}) async {
    final query = select(cachedConversations)
      ..orderBy([(c) => OrderingTerm.desc(c.lastMessageAt)]);
    if (!includeArchived) {
      query.where((c) => c.isArchived.equals(false));
    }
    return query.get();
  }

  Future<void> updateUnreadCount(String oderId, int count) async {
    await (update(cachedConversations)..where((c) => c.oderId.equals(oderId)))
        .write(CachedConversationsCompanion(unreadCount: Value(count)));
  }

  Future<void> incrementUnreadCount(String oderId) async {
    await customStatement(
      'UPDATE cached_conversations SET unread_count = unread_count + 1 WHERE oder_id = ?',
      [oderId],
    );
  }

  // Contact operations
  Future<void> saveContact(CachedContact contact) async {
    await into(cachedContacts).insertOnConflictUpdate(contact);
  }

  Future<void> saveContacts(List<CachedContact> contacts) async {
    await batch((batch) {
      batch.insertAllOnConflictUpdate(cachedContacts, contacts);
    });
  }

  Future<List<CachedContact>> getContacts() async {
    final query = select(cachedContacts)
      ..where((c) => c.isBlocked.equals(false))
      ..orderBy([(c) => OrderingTerm.asc(c.displayName)]);
    return query.get();
  }

  Future<CachedContact?> getContact(String id) async {
    final query = select(cachedContacts)..where((c) => c.id.equals(id));
    return query.getSingleOrNull();
  }

  // Pending message operations
  Future<int> addPendingMessage(PendingMessagesCompanion message) async {
    return into(pendingMessages).insert(message);
  }

  Future<List<PendingMessage>> getPendingMessages() async {
    final query = select(pendingMessages)
      ..orderBy([(m) => OrderingTerm.asc(m.createdAt)]);
    return query.get();
  }

  Future<void> removePendingMessage(int id) async {
    await (delete(pendingMessages)..where((m) => m.id.equals(id))).go();
  }

  Future<void> incrementRetryCount(int id) async {
    await customStatement(
      'UPDATE pending_messages SET retry_count = retry_count + 1 WHERE id = ?',
      [id],
    );
  }

  // Sync state operations
  Future<void> setSyncState(String key, String value) async {
    await into(syncState).insertOnConflictUpdate(
      SyncStateCompanion(
        key: Value(key),
        value: Value(value),
        updatedAt: Value(DateTime.now()),
      ),
    );
  }

  Future<String?> getSyncState(String key) async {
    final query = select(syncState)..where((s) => s.key.equals(key));
    final result = await query.getSingleOrNull();
    return result?.value;
  }

  Future<DateTime?> getLastSyncTime() async {
    final value = await getSyncState('last_sync');
    if (value == null) return null;
    return DateTime.tryParse(value);
  }

  Future<void> setLastSyncTime(DateTime time) async {
    await setSyncState('last_sync', time.toIso8601String());
  }

  // Clear all data (for logout)
  Future<void> clearAll() async {
    await delete(cachedMessages).go();
    await delete(cachedConversations).go();
    await delete(cachedContacts).go();
    await delete(pendingMessages).go();
    await delete(syncState).go();
  }
}
