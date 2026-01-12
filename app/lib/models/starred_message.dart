import 'message.dart';
import 'user.dart';
import 'group.dart';

class StarredMessage {
  final String id;
  final Message message;
  final User? user;      // For DM context
  final GroupInfo? group; // For group context
  final DateTime starredAt;

  StarredMessage({
    required this.id,
    required this.message,
    this.user,
    this.group,
    required this.starredAt,
  });

  factory StarredMessage.fromJson(Map<String, dynamic> json) {
    return StarredMessage(
      id: json['id'] as String,
      message: Message.fromJson(json['message'] as Map<String, dynamic>),
      user: json['user'] != null
          ? User.fromJson(json['user'] as Map<String, dynamic>)
          : null,
      group: json['group'] != null
          ? GroupInfo.fromJson(json['group'] as Map<String, dynamic>)
          : null,
      starredAt: DateTime.parse(json['starred_at'] as String),
    );
  }

  String get conversationName {
    if (group != null) {
      return group!.name;
    }
    if (user != null) {
      return user!.displayNameOrUsername;
    }
    return 'Unknown';
  }

  bool get isGroupMessage => group != null;
}

class GroupInfo {
  final String id;
  final String name;

  GroupInfo({required this.id, required this.name});

  factory GroupInfo.fromJson(Map<String, dynamic> json) {
    return GroupInfo(
      id: json['id'] as String,
      name: json['name'] as String,
    );
  }
}
