import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/group.dart';
import '../models/message.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class GroupsState {
  final List<Group> groups;
  final Map<String, List<Message>> messages; // keyed by group ID
  final bool isLoading;
  final String? error;

  GroupsState({
    this.groups = const [],
    this.messages = const {},
    this.isLoading = false,
    this.error,
  });

  GroupsState copyWith({
    List<Group>? groups,
    Map<String, List<Message>>? messages,
    bool? isLoading,
    String? error,
  }) {
    return GroupsState(
      groups: groups ?? this.groups,
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class GroupsNotifier extends StateNotifier<GroupsState> {
  final ApiService _apiService;

  GroupsNotifier(this._apiService) : super(GroupsState());

  Future<void> loadGroups() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final groups = await _apiService.getGroups();
      state = state.copyWith(groups: groups, isLoading: false);
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to load groups: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<Group?> createGroup({
    required String name,
    String? description,
    List<String>? memberIds,
  }) async {
    try {
      final group = await _apiService.createGroup(
        name: name,
        description: description,
        memberIds: memberIds,
      );
      state = state.copyWith(groups: [...state.groups, group]);
      return group;
    } catch (e) {
      state = state.copyWith(error: 'Failed to create group: ${e.toString()}');
      return null;
    }
  }

  Future<Group?> getGroupDetails(String groupId) async {
    try {
      final group = await _apiService.getGroup(groupId);
      // Update the group in the list
      state = state.copyWith(
        groups: state.groups.map((g) => g.id == groupId ? group : g).toList(),
      );
      return group;
    } catch (e) {
      state = state.copyWith(error: 'Failed to get group: ${e.toString()}');
      return null;
    }
  }

  Future<void> loadGroupMessages(String groupId) async {
    try {
      final messages = await _apiService.getGroupMessages(groupId);
      state = state.copyWith(
        messages: {...state.messages, groupId: messages},
      );
    } catch (e) {
      state = state.copyWith(error: 'Failed to load messages');
    }
  }

  Future<void> addMember(String groupId, String userId) async {
    try {
      await _apiService.addGroupMember(groupId, userId);
      // Refresh group details
      await getGroupDetails(groupId);
    } catch (e) {
      state = state.copyWith(error: 'Failed to add member: ${e.toString()}');
    }
  }

  Future<void> removeMember(String groupId, String userId) async {
    try {
      await _apiService.removeGroupMember(groupId, userId);
      // Refresh group details
      await getGroupDetails(groupId);
    } catch (e) {
      state = state.copyWith(error: 'Failed to remove member: ${e.toString()}');
    }
  }

  Future<void> leaveGroup(String groupId) async {
    try {
      await _apiService.leaveGroup(groupId);
      state = state.copyWith(
        groups: state.groups.where((g) => g.id != groupId).toList(),
      );
    } catch (e) {
      state = state.copyWith(error: 'Failed to leave group: ${e.toString()}');
    }
  }

  void addMessage(Message message) {
    if (message.groupId == null) return;

    final groupId = message.groupId!;
    final currentMessages = state.messages[groupId] ?? [];
    state = state.copyWith(
      messages: {...state.messages, groupId: [message, ...currentMessages]},
    );
  }

  void handleGroupEvent(Map<String, dynamic> event) {
    final type = event['type'] as String?;
    final groupId = event['group_id'] as String?;

    if (groupId == null) return;

    switch (type) {
      case 'group_added':
        // Refresh groups list
        loadGroups();
        break;
      case 'group_removed':
        state = state.copyWith(
          groups: state.groups.where((g) => g.id != groupId).toList(),
        );
        break;
      case 'member_joined':
      case 'member_left':
        // Refresh group details
        getGroupDetails(groupId);
        break;
    }
  }

  void updateMessageReactions(String messageId, List<dynamic> reactionsData) {
    final newMessages = <String, List<Message>>{};

    for (final entry in state.messages.entries) {
      newMessages[entry.key] = entry.value.map((msg) {
        if (msg.id == messageId) {
          // Parse reactions - will need to import reaction model
          return msg;
        }
        return msg;
      }).toList();
    }

    state = state.copyWith(messages: newMessages);
  }
}

final groupsProvider = StateNotifierProvider<GroupsNotifier, GroupsState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return GroupsNotifier(apiService);
});
