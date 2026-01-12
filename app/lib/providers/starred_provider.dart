import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/starred_message.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class StarredState {
  final List<StarredMessage> messages;
  final Set<String> starredIds; // For quick lookup
  final bool isLoading;
  final String? error;

  StarredState({
    this.messages = const [],
    this.starredIds = const {},
    this.isLoading = false,
    this.error,
  });

  StarredState copyWith({
    List<StarredMessage>? messages,
    Set<String>? starredIds,
    bool? isLoading,
    String? error,
  }) {
    return StarredState(
      messages: messages ?? this.messages,
      starredIds: starredIds ?? this.starredIds,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  bool isStarred(String messageId) => starredIds.contains(messageId);
}

class StarredNotifier extends StateNotifier<StarredState> {
  final ApiService _apiService;

  StarredNotifier(this._apiService) : super(StarredState());

  Future<void> loadStarredMessages() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final messages = await _apiService.getStarredMessages();
      final starredIds = messages.map((m) => m.message.id).toSet();
      state = state.copyWith(
        messages: messages,
        starredIds: starredIds,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to load starred messages',
        isLoading: false,
      );
    }
  }

  Future<bool> toggleStar(String messageId) async {
    final isCurrentlyStarred = state.isStarred(messageId);

    try {
      if (isCurrentlyStarred) {
        await _apiService.unstarMessage(messageId);
        state = state.copyWith(
          messages: state.messages.where((m) => m.message.id != messageId).toList(),
          starredIds: {...state.starredIds}..remove(messageId),
        );
        return false;
      } else {
        await _apiService.starMessage(messageId);
        state = state.copyWith(
          starredIds: {...state.starredIds, messageId},
        );
        // Reload to get full message details
        await loadStarredMessages();
        return true;
      }
    } catch (e) {
      return isCurrentlyStarred; // Return previous state on error
    }
  }

  Future<void> starMessage(String messageId) async {
    if (state.isStarred(messageId)) return;

    try {
      await _apiService.starMessage(messageId);
      state = state.copyWith(
        starredIds: {...state.starredIds, messageId},
      );
    } catch (e) {
      // Ignore error
    }
  }

  Future<void> unstarMessage(String messageId) async {
    if (!state.isStarred(messageId)) return;

    try {
      await _apiService.unstarMessage(messageId);
      state = state.copyWith(
        messages: state.messages.where((m) => m.message.id != messageId).toList(),
        starredIds: {...state.starredIds}..remove(messageId),
      );
    } catch (e) {
      // Ignore error
    }
  }
}

final starredProvider = StateNotifierProvider<StarredNotifier, StarredState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return StarredNotifier(apiService);
});
