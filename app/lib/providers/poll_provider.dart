import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/poll.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class PollState {
  final Map<String, Poll> polls; // pollId -> Poll
  final bool isLoading;
  final String? error;

  PollState({
    this.polls = const {},
    this.isLoading = false,
    this.error,
  });

  PollState copyWith({
    Map<String, Poll>? polls,
    bool? isLoading,
    String? error,
  }) {
    return PollState(
      polls: polls ?? this.polls,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  Poll? getPoll(String pollId) => polls[pollId];
}

class PollNotifier extends StateNotifier<PollState> {
  final ApiService _apiService;

  PollNotifier(this._apiService) : super(PollState());

  Future<Poll?> createPoll({
    required String question,
    required List<String> options,
    bool multiSelect = false,
    bool anonymous = false,
    String? groupId,
    String? recipientId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final poll = await _apiService.createPoll(
        question: question,
        options: options,
        multiSelect: multiSelect,
        anonymous: anonymous,
        groupId: groupId,
        recipientId: recipientId,
      );

      state = state.copyWith(
        polls: {...state.polls, poll.id: poll},
        isLoading: false,
      );
      return poll;
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to create poll',
        isLoading: false,
      );
      return null;
    }
  }

  Future<Poll?> loadPoll(String pollId) async {
    try {
      final poll = await _apiService.getPoll(pollId);
      state = state.copyWith(
        polls: {...state.polls, poll.id: poll},
      );
      return poll;
    } catch (e) {
      return null;
    }
  }

  Future<Poll?> vote(String pollId, String optionId) async {
    try {
      final poll = await _apiService.votePoll(pollId, optionId);
      state = state.copyWith(
        polls: {...state.polls, poll.id: poll},
      );
      return poll;
    } catch (e) {
      return null;
    }
  }

  Future<Poll?> closePoll(String pollId) async {
    try {
      final poll = await _apiService.closePoll(pollId);
      state = state.copyWith(
        polls: {...state.polls, poll.id: poll},
      );
      return poll;
    } catch (e) {
      return null;
    }
  }

  void updatePoll(Poll poll) {
    state = state.copyWith(
      polls: {...state.polls, poll.id: poll},
    );
  }
}

final pollProvider = StateNotifierProvider<PollNotifier, PollState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return PollNotifier(apiService);
});
