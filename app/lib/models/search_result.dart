import 'message.dart';
import 'user.dart';

class SearchResult {
  final Message message;
  final User? user;      // For DM results
  final GroupInfo? group; // For group message results

  SearchResult({
    required this.message,
    this.user,
    this.group,
  });

  factory SearchResult.fromJson(Map<String, dynamic> json) {
    return SearchResult(
      message: Message.fromJson(json['message'] as Map<String, dynamic>),
      user: json['user'] != null
          ? User.fromJson(json['user'] as Map<String, dynamic>)
          : null,
      group: json['group'] != null
          ? GroupInfo.fromJson(json['group'] as Map<String, dynamic>)
          : null,
    );
  }

  bool get isGroupMessage => group != null;
  bool get isDM => user != null;

  String get conversationName {
    if (group != null) return group!.name;
    if (user != null) return user!.displayNameOrUsername;
    return 'Unknown';
  }
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
