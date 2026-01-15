class PinnedMessage {
  final String id;
  final String messageId;
  final String? groupId;
  final String? conversationKey;
  final String pinnedById;
  final DateTime pinnedAt;
  final String? messageContent;
  final String? senderName;

  PinnedMessage({
    required this.id,
    required this.messageId,
    this.groupId,
    this.conversationKey,
    required this.pinnedById,
    required this.pinnedAt,
    this.messageContent,
    this.senderName,
  });

  factory PinnedMessage.fromJson(Map<String, dynamic> json) {
    return PinnedMessage(
      id: json['id'] ?? '',
      messageId: json['message_id'] ?? '',
      groupId: json['group_id'],
      conversationKey: json['conversation_key'],
      pinnedById: json['pinned_by_id'] ?? '',
      pinnedAt: json['pinned_at'] != null
          ? DateTime.parse(json['pinned_at'])
          : DateTime.now(),
      messageContent: json['message_content'],
      senderName: json['sender_name'],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'message_id': messageId,
      'group_id': groupId,
      'conversation_key': conversationKey,
      'pinned_by_id': pinnedById,
      'pinned_at': pinnedAt.toIso8601String(),
      'message_content': messageContent,
      'sender_name': senderName,
    };
  }
}
