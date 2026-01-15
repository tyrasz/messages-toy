class User {
  final String id;
  final String username;
  final String? displayName;
  final String? avatarUrl;
  final String? about;
  final String? statusEmoji;
  final DateTime? lastSeen;
  final bool online;

  User({
    required this.id,
    required this.username,
    this.displayName,
    this.avatarUrl,
    this.about,
    this.statusEmoji,
    this.lastSeen,
    this.online = false,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as String,
      username: json['username'] as String,
      displayName: json['display_name'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      about: json['about'] as String?,
      statusEmoji: json['status_emoji'] as String?,
      lastSeen: json['last_seen'] != null
          ? DateTime.parse(json['last_seen'] as String)
          : null,
      online: json['online'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'username': username,
      'display_name': displayName,
      'avatar_url': avatarUrl,
      'about': about,
      'status_emoji': statusEmoji,
      'last_seen': lastSeen?.toIso8601String(),
      'online': online,
    };
  }

  User copyWith({
    String? id,
    String? username,
    String? displayName,
    String? avatarUrl,
    String? about,
    String? statusEmoji,
    DateTime? lastSeen,
    bool? online,
  }) {
    return User(
      id: id ?? this.id,
      username: username ?? this.username,
      displayName: displayName ?? this.displayName,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      about: about ?? this.about,
      statusEmoji: statusEmoji ?? this.statusEmoji,
      lastSeen: lastSeen ?? this.lastSeen,
      online: online ?? this.online,
    );
  }

  String get displayNameOrUsername => displayName ?? username;
}
