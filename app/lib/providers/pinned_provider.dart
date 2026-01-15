import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/pinned_message.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class PinnedState {
  // Key is either groupId or "dm:userId1:userId2" for DMs
  final Map<String, PinnedMessage> pinnedMessages;
  final bool isLoading;
  final String? error;

  PinnedState({
    this.pinnedMessages = const {},
    this.isLoading = false,
    this.error,
  });

  PinnedState copyWith({
    Map<String, PinnedMessage>? pinnedMessages,
    bool? isLoading,
    String? error,
  }) {
    return PinnedState(
      pinnedMessages: pinnedMessages ?? this.pinnedMessages,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  PinnedMessage? getForGroup(String groupId) => pinnedMessages[groupId];

  PinnedMessage? getForDm(String otherUserId, String currentUserId) {
    // Try both key orders
    final key1 = 'dm:$currentUserId:$otherUserId';
    final key2 = 'dm:$otherUserId:$currentUserId';
    return pinnedMessages[key1] ?? pinnedMessages[key2];
  }
}

class PinnedNotifier extends StateNotifier<PinnedState> {
  final ApiService _apiService;

  PinnedNotifier(this._apiService) : super(PinnedState());

  String _getDmKey(String userId1, String userId2) {
    // Sort to ensure consistent key
    final sorted = [userId1, userId2]..sort();
    return 'dm:${sorted[0]}:${sorted[1]}';
  }

  Future<PinnedMessage?> loadPinnedMessage({
    String? groupId,
    String? otherUserId,
    String? currentUserId,
  }) async {
    try {
      final pinned = await _apiService.getPinnedMessage(
        groupId: groupId,
        otherUserId: otherUserId,
      );

      if (pinned != null) {
        final key = groupId ?? _getDmKey(otherUserId!, currentUserId!);
        state = state.copyWith(
          pinnedMessages: {...state.pinnedMessages, key: pinned},
        );
      }
      return pinned;
    } catch (e) {
      return null;
    }
  }

  Future<PinnedMessage?> pinMessage({
    required String messageId,
    String? groupId,
    String? otherUserId,
    String? currentUserId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final pinned = await _apiService.pinMessage(
        messageId: messageId,
        groupId: groupId,
        otherUserId: otherUserId,
      );

      final key = groupId ?? _getDmKey(otherUserId!, currentUserId!);
      state = state.copyWith(
        pinnedMessages: {...state.pinnedMessages, key: pinned},
        isLoading: false,
      );
      return pinned;
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to pin message',
        isLoading: false,
      );
      return null;
    }
  }

  Future<void> unpinMessage({
    String? groupId,
    String? otherUserId,
    String? currentUserId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      await _apiService.unpinMessage(
        groupId: groupId,
        otherUserId: otherUserId,
      );

      final key = groupId ?? _getDmKey(otherUserId!, currentUserId!);
      final updated = Map<String, PinnedMessage>.from(state.pinnedMessages);
      updated.remove(key);

      state = state.copyWith(
        pinnedMessages: updated,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to unpin message',
        isLoading: false,
      );
    }
  }

  void updatePinned({
    String? groupId,
    String? otherUserId,
    String? currentUserId,
    PinnedMessage? pinned,
  }) {
    final key = groupId ?? _getDmKey(otherUserId!, currentUserId!);

    if (pinned != null) {
      state = state.copyWith(
        pinnedMessages: {...state.pinnedMessages, key: pinned},
      );
    } else {
      final updated = Map<String, PinnedMessage>.from(state.pinnedMessages);
      updated.remove(key);
      state = state.copyWith(pinnedMessages: updated);
    }
  }
}

final pinnedProvider = StateNotifierProvider<PinnedNotifier, PinnedState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return PinnedNotifier(apiService);
});
