import 'dart:async';
import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/foundation.dart';

enum NetworkStatus {
  online,
  offline,
  unknown,
}

class ConnectivityService {
  final Connectivity _connectivity = Connectivity();
  final _statusController = StreamController<NetworkStatus>.broadcast();

  NetworkStatus _currentStatus = NetworkStatus.unknown;
  StreamSubscription<List<ConnectivityResult>>? _subscription;

  Stream<NetworkStatus> get statusStream => _statusController.stream;
  NetworkStatus get currentStatus => _currentStatus;
  bool get isOnline => _currentStatus == NetworkStatus.online;

  Future<void> initialize() async {
    // Get initial status
    final results = await _connectivity.checkConnectivity();
    _updateStatus(results);

    // Listen for changes
    _subscription = _connectivity.onConnectivityChanged.listen(_updateStatus);
  }

  void _updateStatus(List<ConnectivityResult> results) {
    NetworkStatus newStatus;

    if (results.contains(ConnectivityResult.none) || results.isEmpty) {
      newStatus = NetworkStatus.offline;
    } else if (results.contains(ConnectivityResult.wifi) ||
               results.contains(ConnectivityResult.mobile) ||
               results.contains(ConnectivityResult.ethernet)) {
      newStatus = NetworkStatus.online;
    } else {
      newStatus = NetworkStatus.unknown;
    }

    if (newStatus != _currentStatus) {
      _currentStatus = newStatus;
      _statusController.add(newStatus);

      if (kDebugMode) {
        print('Network status changed: $newStatus');
      }
    }
  }

  Future<bool> checkConnection() async {
    final results = await _connectivity.checkConnectivity();
    _updateStatus(results);
    return isOnline;
  }

  void dispose() {
    _subscription?.cancel();
    _statusController.close();
  }
}
