import 'message.dart';
import 'user.dart';

class Conversation {
  final User user;
  final Message? lastMessage;
  final int unreadCount;

  Conversation({
    required this.user,
    this.lastMessage,
    this.unreadCount = 0,
  });

  factory Conversation.fromJson(Map<String, dynamic> json) {
    return Conversation(
      user: User.fromJson(json['user'] as Map<String, dynamic>),
      lastMessage: json['last_message'] != null
          ? Message.fromJson(json['last_message'] as Map<String, dynamic>)
          : null,
      unreadCount: json['unread_count'] as int? ?? 0,
    );
  }
}
