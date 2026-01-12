import 'user.dart';

class BlockedUser {
  final String id;
  final String blockedId;
  final User blockedUser;
  final DateTime createdAt;

  BlockedUser({
    required this.id,
    required this.blockedId,
    required this.blockedUser,
    required this.createdAt,
  });

  factory BlockedUser.fromJson(Map<String, dynamic> json) {
    return BlockedUser(
      id: json['id'] as String,
      blockedId: json['blocked_id'] as String,
      blockedUser: User.fromJson(json['blocked_user'] as Map<String, dynamic>),
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  String get displayName => blockedUser.displayNameOrUsername;
}
