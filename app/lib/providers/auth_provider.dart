import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/user.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';
import '../services/offline_database.dart';
import '../services/sync_service.dart';
import '../services/push_service.dart';

final apiServiceProvider = Provider<ApiService>((ref) {
  return ApiService();
});

final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  return WebSocketService();
});

final offlineDatabaseProvider = Provider<OfflineDatabase>((ref) {
  return OfflineDatabase();
});

final syncServiceProvider = Provider<SyncService>((ref) {
  final db = ref.watch(offlineDatabaseProvider);
  final api = ref.watch(apiServiceProvider);
  final ws = ref.watch(webSocketServiceProvider);
  return SyncService(db: db, api: api, ws: ws);
});

final pushServiceProvider = Provider<PushService>((ref) {
  final api = ref.watch(apiServiceProvider);
  return PushService(api: api);
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
  final SyncService _syncService;
  final PushService _pushService;
  final OfflineDatabase _offlineDb;

  AuthNotifier(
    this._apiService,
    this._webSocketService,
    this._syncService,
    this._pushService,
    this._offlineDb,
  ) : super(AuthState()) {
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
      _initializePushNotifications();
      // Trigger initial sync
      _syncService.sync();
    } else {
      state = state.copyWith(status: AuthStatus.unauthenticated);
    }
  }

  Future<void> _initializePushNotifications() async {
    try {
      await _pushService.initialize();
      await _pushService.requestPermission();
    } catch (e) {
      // Push notifications are optional, don't fail auth
      print('Push notification setup failed: $e');
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
      _initializePushNotifications();
      _syncService.sync();
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
      _initializePushNotifications();
      _syncService.sync();
    } catch (e) {
      state = state.copyWith(
        error: 'Registration failed: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<void> logout() async {
    _webSocketService.disconnect();
    await _pushService.unregisterToken();
    await _syncService.clearCache();
    await _apiService.logout();
    state = AuthState(status: AuthStatus.unauthenticated);
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  final webSocketService = ref.watch(webSocketServiceProvider);
  final syncService = ref.watch(syncServiceProvider);
  final pushService = ref.watch(pushServiceProvider);
  final offlineDb = ref.watch(offlineDatabaseProvider);
  return AuthNotifier(
    apiService,
    webSocketService,
    syncService,
    pushService,
    offlineDb,
  );
});
