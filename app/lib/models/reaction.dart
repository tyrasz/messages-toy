class ReactionInfo {
  final String emoji;
  final int count;
  final List<String> users; // User IDs who reacted

  ReactionInfo({
    required this.emoji,
    required this.count,
    required this.users,
  });

  factory ReactionInfo.fromJson(Map<String, dynamic> json) {
    return ReactionInfo(
      emoji: json['emoji'] as String,
      count: json['count'] as int,
      users: (json['users'] as List<dynamic>?)?.cast<String>() ?? [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'emoji': emoji,
      'count': count,
      'users': users,
    };
  }

  bool hasUserReacted(String userId) => users.contains(userId);
}

class ReactionEvent {
  final String messageId;
  final String userId;
  final String? emoji;
  final String action; // "added" or "removed"
  final List<ReactionInfo> reactions;

  ReactionEvent({
    required this.messageId,
    required this.userId,
    this.emoji,
    required this.action,
    required this.reactions,
  });

  factory ReactionEvent.fromJson(Map<String, dynamic> json) {
    return ReactionEvent(
      messageId: json['message_id'] as String,
      userId: json['user_id'] as String,
      emoji: json['emoji'] as String?,
      action: json['action'] as String,
      reactions: (json['reactions'] as List<dynamic>?)
              ?.map((r) => ReactionInfo.fromJson(r as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

// Common emoji reactions for quick access
class QuickReactions {
  static const List<String> defaults = [
    '\u{1F44D}', // 👍
    '\u{2764}',  // ❤️
    '\u{1F602}', // 😂
    '\u{1F62E}', // 😮
    '\u{1F622}', // 😢
    '\u{1F64F}', // 🙏
  ];
}
