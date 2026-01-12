import 'user.dart';

enum GroupRole { owner, admin, member }

class GroupMember {
  final String id;
  final String groupId;
  final String userId;
  final GroupRole role;
  final User user;
  final DateTime joinedAt;

  GroupMember({
    required this.id,
    required this.groupId,
    required this.userId,
    required this.role,
    required this.user,
    required this.joinedAt,
  });

  factory GroupMember.fromJson(Map<String, dynamic> json) {
    return GroupMember(
      id: json['id'] as String,
      groupId: json['group_id'] as String,
      userId: json['user_id'] as String,
      role: _parseRole(json['role'] as String?),
      user: User.fromJson(json['user'] as Map<String, dynamic>),
      joinedAt: DateTime.parse(json['joined_at'] as String),
    );
  }

  static GroupRole _parseRole(String? role) {
    switch (role) {
      case 'owner':
        return GroupRole.owner;
      case 'admin':
        return GroupRole.admin;
      default:
        return GroupRole.member;
    }
  }

  bool get isAdmin => role == GroupRole.admin || role == GroupRole.owner;
  bool get isOwner => role == GroupRole.owner;
}

class Group {
  final String id;
  final String name;
  final String? description;
  final String? avatarUrl;
  final String createdBy;
  final int memberCount;
  final List<GroupMember>? members;
  final GroupRole? myRole;
  final DateTime createdAt;

  Group({
    required this.id,
    required this.name,
    this.description,
    this.avatarUrl,
    required this.createdBy,
    this.memberCount = 0,
    this.members,
    this.myRole,
    required this.createdAt,
  });

  factory Group.fromJson(Map<String, dynamic> json) {
    return Group(
      id: json['id'] as String,
      name: json['name'] as String,
      description: json['description'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      createdBy: json['created_by'] as String,
      memberCount: json['member_count'] as int? ?? 0,
      members: json['members'] != null
          ? (json['members'] as List)
              .map((m) => GroupMember.fromJson(m as Map<String, dynamic>))
              .toList()
          : null,
      myRole: json['my_role'] != null
          ? GroupMember._parseRole(json['my_role'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  bool get canManageMembers =>
      myRole == GroupRole.owner || myRole == GroupRole.admin;
}
