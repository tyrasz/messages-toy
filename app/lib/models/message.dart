import 'reaction.dart';

enum MessageStatus { sent, delivered, read }

enum MediaType { none, image, video, audio, document }

class ReplyPreview {
  final String id;
  final String senderId;
  final String? content;

  ReplyPreview({
    required this.id,
    required this.senderId,
    this.content,
  });

  factory ReplyPreview.fromJson(Map<String, dynamic> json) {
    return ReplyPreview(
      id: json['id'] as String,
      senderId: json['sender_id'] as String,
      content: json['content'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'sender_id': senderId,
      if (content != null) 'content': content,
    };
  }
}

class Message {
  final String id;
  final String senderId;
  final String? recipientId; // For DMs
  final String? groupId;     // For group messages
  final String? replyToId;   // ID of message being replied to
  final ReplyPreview? replyTo; // Preview of replied message
  final String? content;
  final String? mediaId;
  final String? mediaUrl;
  final String? mediaContentType;
  final MediaType mediaType;
  final String? thumbnailUrl;
  final int? mediaDuration;      // Duration in seconds for audio/video
  final int? mediaWidth;
  final int? mediaHeight;
  final int? pageCount;          // For documents
  final MessageStatus status;
  final DateTime? editedAt;  // When message was edited
  final bool isDeleted;      // Whether message is deleted for everyone
  final List<ReactionInfo> reactions; // Reactions on this message
  final String? forwardedFrom; // Original sender name if forwarded
  final DateTime? expiresAt;   // For disappearing messages
  final DateTime createdAt;

  Message({
    required this.id,
    required this.senderId,
    this.recipientId,
    this.groupId,
    this.replyToId,
    this.replyTo,
    this.content,
    this.mediaId,
    this.mediaUrl,
    this.mediaContentType,
    this.mediaType = MediaType.none,
    this.thumbnailUrl,
    this.mediaDuration,
    this.mediaWidth,
    this.mediaHeight,
    this.pageCount,
    this.status = MessageStatus.sent,
    this.editedAt,
    this.isDeleted = false,
    this.reactions = const [],
    this.forwardedFrom,
    this.expiresAt,
    required this.createdAt,
  });

  bool get isEdited => editedAt != null;
  bool get isDisappearing => expiresAt != null;
  bool get isExpired => expiresAt != null && expiresAt!.isBefore(DateTime.now());
  bool get hasMedia => mediaId != null;
  bool get isImageMessage => mediaType == MediaType.image;
  bool get isVideoMessage => mediaType == MediaType.video;
  bool get isAudioMessage => mediaType == MediaType.audio;
  bool get isDocumentMessage => mediaType == MediaType.document;
  bool get hasReactions => reactions.isNotEmpty;
  bool get isForwarded => forwardedFrom != null;
  String get displayContent => isDeleted ? '[Message deleted]' : (content ?? '');

  factory Message.fromJson(Map<String, dynamic> json) {
    final media = json['media'] as Map<String, dynamic>?;
    return Message(
      id: json['id'] as String,
      senderId: json['sender_id'] ?? json['from'] as String,
      recipientId: json['recipient_id'] as String? ?? json['to'] as String?,
      groupId: json['group_id'] as String?,
      replyToId: json['reply_to_id'] as String?,
      replyTo: json['reply_to'] != null
          ? ReplyPreview.fromJson(json['reply_to'] as Map<String, dynamic>)
          : null,
      content: json['content'] as String?,
      mediaId: json['media_id'] as String?,
      mediaUrl: media?['url'] as String?,
      mediaContentType: media?['content_type'] as String?,
      mediaType: _parseMediaType(media?['media_type'] as String?),
      thumbnailUrl: media?['thumbnail_url'] as String?,
      mediaDuration: media?['duration'] as int?,
      mediaWidth: media?['width'] as int?,
      mediaHeight: media?['height'] as int?,
      pageCount: media?['page_count'] as int?,
      status: parseStatus(json['status'] as String?),
      editedAt: json['edited_at'] != null
          ? DateTime.parse(json['edited_at'] as String)
          : null,
      isDeleted: json['deleted_at'] != null,
      reactions: (json['reactions'] as List<dynamic>?)
              ?.map((r) => ReactionInfo.fromJson(r as Map<String, dynamic>))
              .toList() ??
          [],
      forwardedFrom: json['forwarded_from'] as String?,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  static MessageStatus parseStatus(String? status) {
    switch (status) {
      case 'delivered':
        return MessageStatus.delivered;
      case 'read':
        return MessageStatus.read;
      default:
        return MessageStatus.sent;
    }
  }

  static MediaType _parseMediaType(String? type) {
    switch (type) {
      case 'image':
        return MediaType.image;
      case 'video':
        return MediaType.video;
      case 'audio':
        return MediaType.audio;
      case 'document':
        return MediaType.document;
      default:
        return MediaType.none;
    }
  }

  bool get isGroupMessage => groupId != null && groupId!.isNotEmpty;
  bool get isReply => replyToId != null || replyTo != null;

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'sender_id': senderId,
      if (recipientId != null) 'recipient_id': recipientId,
      if (groupId != null) 'group_id': groupId,
      if (replyToId != null) 'reply_to_id': replyToId,
      if (replyTo != null) 'reply_to': replyTo!.toJson(),
      'content': content,
      'media_id': mediaId,
      if (forwardedFrom != null) 'forwarded_from': forwardedFrom,
      'status': status.name,
      'created_at': createdAt.toIso8601String(),
    };
  }

  Message copyWith({
    String? id,
    String? senderId,
    String? recipientId,
    String? groupId,
    String? replyToId,
    ReplyPreview? replyTo,
    String? content,
    String? mediaId,
    String? mediaUrl,
    String? mediaContentType,
    MediaType? mediaType,
    String? thumbnailUrl,
    int? mediaDuration,
    int? mediaWidth,
    int? mediaHeight,
    int? pageCount,
    MessageStatus? status,
    DateTime? editedAt,
    bool? isDeleted,
    List<ReactionInfo>? reactions,
    String? forwardedFrom,
    DateTime? expiresAt,
    DateTime? createdAt,
  }) {
    return Message(
      id: id ?? this.id,
      senderId: senderId ?? this.senderId,
      recipientId: recipientId ?? this.recipientId,
      groupId: groupId ?? this.groupId,
      replyToId: replyToId ?? this.replyToId,
      replyTo: replyTo ?? this.replyTo,
      content: content ?? this.content,
      mediaId: mediaId ?? this.mediaId,
      mediaUrl: mediaUrl ?? this.mediaUrl,
      mediaContentType: mediaContentType ?? this.mediaContentType,
      mediaType: mediaType ?? this.mediaType,
      thumbnailUrl: thumbnailUrl ?? this.thumbnailUrl,
      mediaDuration: mediaDuration ?? this.mediaDuration,
      mediaWidth: mediaWidth ?? this.mediaWidth,
      mediaHeight: mediaHeight ?? this.mediaHeight,
      pageCount: pageCount ?? this.pageCount,
      status: status ?? this.status,
      editedAt: editedAt ?? this.editedAt,
      isDeleted: isDeleted ?? this.isDeleted,
      reactions: reactions ?? this.reactions,
      forwardedFrom: forwardedFrom ?? this.forwardedFrom,
      expiresAt: expiresAt ?? this.expiresAt,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
