import 'user.dart';

class Contact {
  final String id;
  final String contactId;
  final String? nickname;
  final User user;
  final DateTime createdAt;

  Contact({
    required this.id,
    required this.contactId,
    this.nickname,
    required this.user,
    required this.createdAt,
  });

  factory Contact.fromJson(Map<String, dynamic> json) {
    return Contact(
      id: json['id'] as String,
      contactId: json['contact_id'] as String,
      nickname: json['nickname'] as String?,
      user: User.fromJson(json['user'] as Map<String, dynamic>),
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  String get displayName => nickname ?? user.displayNameOrUsername;
  bool get isOnline => user.online;
}
