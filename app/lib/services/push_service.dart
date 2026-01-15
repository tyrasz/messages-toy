import 'dart:async';
import 'package:flutter/foundation.dart';
import 'api_service.dart';

/// Service for managing push notifications across platforms
class PushService {
  final ApiService _api;

  String? _token;
  final _notificationController = StreamController<PushNotification>.broadcast();

  Stream<PushNotification> get notificationStream => _notificationController.stream;
  String? get token => _token;

  PushService({required ApiService api}) : _api = api;

  /// Initialize push notifications
  Future<void> initialize() async {
    if (kIsWeb) {
      await _initializeWeb();
    } else {
      await _initializeMobile();
    }
  }

  /// Initialize for web platform
  Future<void> _initializeWeb() async {
    // Web push is handled by the service worker
    // Token registration happens via Firebase JS SDK in the web app
    print('Push notifications: Web platform - using service worker');
  }

  /// Initialize for mobile platforms (iOS/Android)
  Future<void> _initializeMobile() async {
    // This would use firebase_messaging package
    // For now, just log - actual implementation needs Firebase setup
    print('Push notifications: Mobile platform - Firebase required');
  }

  /// Register device token with backend
  Future<void> registerToken(String token, String platform) async {
    try {
      _token = token;
      // Call your API to register the token
      // await _api.registerPushToken(token: token, platform: platform);
      print('Push token registered: $platform');
    } catch (e) {
      print('Failed to register push token: $e');
    }
  }

  /// Unregister device token
  Future<void> unregisterToken() async {
    if (_token == null) return;

    try {
      // await _api.unregisterPushToken(token: _token!);
      _token = null;
      print('Push token unregistered');
    } catch (e) {
      print('Failed to unregister push token: $e');
    }
  }

  /// Handle incoming notification
  void handleNotification(Map<String, dynamic> data) {
    final notification = PushNotification(
      title: data['title'] as String? ?? 'New Message',
      body: data['body'] as String? ?? '',
      conversationId: data['conversation_id'] as String?,
      senderId: data['sender_id'] as String?,
      messageId: data['message_id'] as String?,
      type: PushNotificationType.values.firstWhere(
        (t) => t.name == (data['type'] as String? ?? 'message'),
        orElse: () => PushNotificationType.message,
      ),
    );

    _notificationController.add(notification);
  }

  /// Request notification permission
  Future<bool> requestPermission() async {
    if (kIsWeb) {
      // Web Notification API permission
      // Actual implementation would use JS interop
      return true;
    } else {
      // Mobile - Firebase handles this
      return true;
    }
  }

  void dispose() {
    _notificationController.close();
  }
}

/// Push notification data model
class PushNotification {
  final String title;
  final String body;
  final String? conversationId;
  final String? senderId;
  final String? messageId;
  final PushNotificationType type;

  PushNotification({
    required this.title,
    required this.body,
    this.conversationId,
    this.senderId,
    this.messageId,
    this.type = PushNotificationType.message,
  });
}

enum PushNotificationType {
  message,
  reaction,
  mention,
  groupInvite,
  call,
}
