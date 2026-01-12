enum MessageStatus { sent, delivered, read }

class Message {
  final String id;
  final String senderId;
  final String? recipientId; // For DMs
  final String? groupId;     // For group messages
  final String? content;
  final String? mediaId;
  final String? mediaUrl;
  final MessageStatus status;
  final DateTime createdAt;

  Message({
    required this.id,
    required this.senderId,
    this.recipientId,
    this.groupId,
    this.content,
    this.mediaId,
    this.mediaUrl,
    this.status = MessageStatus.sent,
    required this.createdAt,
  });

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      senderId: json['sender_id'] ?? json['from'] as String,
      recipientId: json['recipient_id'] as String? ?? json['to'] as String?,
      groupId: json['group_id'] as String?,
      content: json['content'] as String?,
      mediaId: json['media_id'] as String?,
      mediaUrl: json['media']?['url'] as String?,
      status: _parseStatus(json['status'] as String?),
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  static MessageStatus _parseStatus(String? status) {
    switch (status) {
      case 'delivered':
        return MessageStatus.delivered;
      case 'read':
        return MessageStatus.read;
      default:
        return MessageStatus.sent;
    }
  }

  bool get isGroupMessage => groupId != null && groupId!.isNotEmpty;

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'sender_id': senderId,
      if (recipientId != null) 'recipient_id': recipientId,
      if (groupId != null) 'group_id': groupId,
      'content': content,
      'media_id': mediaId,
      'status': status.name,
      'created_at': createdAt.toIso8601String(),
    };
  }

  Message copyWith({
    String? id,
    String? senderId,
    String? recipientId,
    String? groupId,
    String? content,
    String? mediaId,
    String? mediaUrl,
    MessageStatus? status,
    DateTime? createdAt,
  }) {
    return Message(
      id: id ?? this.id,
      senderId: senderId ?? this.senderId,
      recipientId: recipientId ?? this.recipientId,
      groupId: groupId ?? this.groupId,
      content: content ?? this.content,
      mediaId: mediaId ?? this.mediaId,
      mediaUrl: mediaUrl ?? this.mediaUrl,
      status: status ?? this.status,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
