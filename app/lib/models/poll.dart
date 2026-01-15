class PollOption {
  final String id;
  final String pollId;
  final String text;
  final int voteCount;
  final List<String> voters;
  final double percentage;
  final bool votedByMe;

  PollOption({
    required this.id,
    required this.pollId,
    required this.text,
    required this.voteCount,
    required this.voters,
    required this.percentage,
    required this.votedByMe,
  });

  factory PollOption.fromJson(Map<String, dynamic> json) {
    return PollOption(
      id: json['id'] ?? '',
      pollId: json['poll_id'] ?? '',
      text: json['text'] ?? '',
      voteCount: json['vote_count'] ?? 0,
      voters: List<String>.from(json['voters'] ?? []),
      percentage: (json['percentage'] ?? 0).toDouble(),
      votedByMe: json['voted_by_me'] ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'poll_id': pollId,
      'text': text,
      'vote_count': voteCount,
      'voters': voters,
      'percentage': percentage,
      'voted_by_me': votedByMe,
    };
  }
}

class Poll {
  final String id;
  final String creatorId;
  final String? messageId;
  final String? groupId;
  final String? recipientId;
  final String question;
  final bool multiSelect;
  final bool anonymous;
  final bool closed;
  final List<PollOption> options;
  final int totalVotes;
  final DateTime createdAt;

  Poll({
    required this.id,
    required this.creatorId,
    this.messageId,
    this.groupId,
    this.recipientId,
    required this.question,
    required this.multiSelect,
    required this.anonymous,
    required this.closed,
    required this.options,
    required this.totalVotes,
    required this.createdAt,
  });

  factory Poll.fromJson(Map<String, dynamic> json) {
    return Poll(
      id: json['id'] ?? '',
      creatorId: json['creator_id'] ?? '',
      messageId: json['message_id'],
      groupId: json['group_id'],
      recipientId: json['recipient_id'],
      question: json['question'] ?? '',
      multiSelect: json['multi_select'] ?? false,
      anonymous: json['anonymous'] ?? false,
      closed: json['closed'] ?? false,
      options: (json['options'] as List<dynamic>?)
              ?.map((o) => PollOption.fromJson(o))
              .toList() ??
          [],
      totalVotes: json['total_votes'] ?? 0,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'creator_id': creatorId,
      'message_id': messageId,
      'group_id': groupId,
      'recipient_id': recipientId,
      'question': question,
      'multi_select': multiSelect,
      'anonymous': anonymous,
      'closed': closed,
      'options': options.map((o) => o.toJson()).toList(),
      'total_votes': totalVotes,
      'created_at': createdAt.toIso8601String(),
    };
  }

  Poll copyWith({
    String? id,
    String? creatorId,
    String? messageId,
    String? groupId,
    String? recipientId,
    String? question,
    bool? multiSelect,
    bool? anonymous,
    bool? closed,
    List<PollOption>? options,
    int? totalVotes,
    DateTime? createdAt,
  }) {
    return Poll(
      id: id ?? this.id,
      creatorId: creatorId ?? this.creatorId,
      messageId: messageId ?? this.messageId,
      groupId: groupId ?? this.groupId,
      recipientId: recipientId ?? this.recipientId,
      question: question ?? this.question,
      multiSelect: multiSelect ?? this.multiSelect,
      anonymous: anonymous ?? this.anonymous,
      closed: closed ?? this.closed,
      options: options ?? this.options,
      totalVotes: totalVotes ?? this.totalVotes,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
