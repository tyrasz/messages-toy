import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/block.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class BlocksState {
  final List<BlockedUser> blockedUsers;
  final bool isLoading;
  final String? error;

  BlocksState({
    this.blockedUsers = const [],
    this.isLoading = false,
    this.error,
  });

  BlocksState copyWith({
    List<BlockedUser>? blockedUsers,
    bool? isLoading,
    String? error,
  }) {
    return BlocksState(
      blockedUsers: blockedUsers ?? this.blockedUsers,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  bool isUserBlocked(String userId) {
    return blockedUsers.any((b) => b.blockedId == userId);
  }
}

class BlocksNotifier extends StateNotifier<BlocksState> {
  final ApiService _apiService;

  BlocksNotifier(this._apiService) : super(BlocksState());

  Future<void> loadBlocks() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final blocks = await _apiService.getBlocks();
      state = state.copyWith(blockedUsers: blocks, isLoading: false);
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to load blocked users: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<bool> blockUser(String userId) async {
    try {
      final block = await _apiService.blockUser(userId);
      state = state.copyWith(
        blockedUsers: [...state.blockedUsers, block],
        error: null,
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to block user: ${e.toString()}',
      );
      return false;
    }
  }

  Future<bool> unblockUser(String userId) async {
    try {
      await _apiService.unblockUser(userId);
      state = state.copyWith(
        blockedUsers: state.blockedUsers.where((b) => b.blockedId != userId).toList(),
        error: null,
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to unblock user: ${e.toString()}',
      );
      return false;
    }
  }

  bool isUserBlocked(String userId) {
    return state.isUserBlocked(userId);
  }

  void clearError() {
    state = state.copyWith(error: null);
  }
}

final blocksProvider = StateNotifierProvider<BlocksNotifier, BlocksState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return BlocksNotifier(apiService);
});
