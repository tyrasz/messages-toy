import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/user.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';

final apiServiceProvider = Provider<ApiService>((ref) {
  return ApiService();
});

final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  return WebSocketService();
});

enum AuthStatus { unknown, authenticated, unauthenticated }

class AuthState {
  final AuthStatus status;
  final User? user;
  final String? error;
  final bool isLoading;

  AuthState({
    this.status = AuthStatus.unknown,
    this.user,
    this.error,
    this.isLoading = false,
  });

  AuthState copyWith({
    AuthStatus? status,
    User? user,
    String? error,
    bool? isLoading,
  }) {
    return AuthState(
      status: status ?? this.status,
      user: user ?? this.user,
      error: error,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiService _apiService;
  final WebSocketService _webSocketService;

  AuthNotifier(this._apiService, this._webSocketService) : super(AuthState()) {
    _init();
  }

  Future<void> _init() async {
    await _apiService.init();
    if (_apiService.isAuthenticated && _apiService.currentUser != null) {
      state = state.copyWith(
        status: AuthStatus.authenticated,
        user: _apiService.currentUser,
      );
      _connectWebSocket();
    } else {
      state = state.copyWith(status: AuthStatus.unauthenticated);
    }
  }

  void _connectWebSocket() {
    if (_apiService.accessToken != null) {
      _webSocketService.connect(_apiService.accessToken!);
    }
  }

  Future<void> login(String username, String password) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final user = await _apiService.login(
        username: username,
        password: password,
      );
      state = state.copyWith(
        status: AuthStatus.authenticated,
        user: user,
        isLoading: false,
      );
      _connectWebSocket();
    } catch (e) {
      state = state.copyWith(
        error: 'Login failed: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<void> register(String username, String password, {String? displayName}) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final user = await _apiService.register(
        username: username,
        password: password,
        displayName: displayName,
      );
      state = state.copyWith(
        status: AuthStatus.authenticated,
        user: user,
        isLoading: false,
      );
      _connectWebSocket();
    } catch (e) {
      state = state.copyWith(
        error: 'Registration failed: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<void> logout() async {
    _webSocketService.disconnect();
    await _apiService.logout();
    state = AuthState(status: AuthStatus.unauthenticated);
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  final webSocketService = ref.watch(webSocketServiceProvider);
  return AuthNotifier(apiService, webSocketService);
});
