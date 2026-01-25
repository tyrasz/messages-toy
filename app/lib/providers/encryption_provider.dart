import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/crypto_service.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

/// Provider for the crypto service singleton
final cryptoServiceProvider = Provider<CryptoService>((ref) {
  return CryptoService();
});

/// E2EE Key registration status
enum KeyRegistrationStatus {
  unknown,
  notRegistered,
  registering,
  registered,
  error,
}

/// State for encryption management
class EncryptionState {
  final KeyRegistrationStatus status;
  final String? deviceId;
  final int prekeyCount;
  final bool needsPreKeyUpload;
  final String? error;
  final Map<String, String> verifiedContacts; // userId -> identityKey

  EncryptionState({
    this.status = KeyRegistrationStatus.unknown,
    this.deviceId,
    this.prekeyCount = 0,
    this.needsPreKeyUpload = false,
    this.error,
    this.verifiedContacts = const {},
  });

  EncryptionState copyWith({
    KeyRegistrationStatus? status,
    String? deviceId,
    int? prekeyCount,
    bool? needsPreKeyUpload,
    String? error,
    Map<String, String>? verifiedContacts,
  }) {
    return EncryptionState(
      status: status ?? this.status,
      deviceId: deviceId ?? this.deviceId,
      prekeyCount: prekeyCount ?? this.prekeyCount,
      needsPreKeyUpload: needsPreKeyUpload ?? this.needsPreKeyUpload,
      error: error,
      verifiedContacts: verifiedContacts ?? this.verifiedContacts,
    );
  }

  bool get isReady => status == KeyRegistrationStatus.registered;
}

/// Manages E2EE encryption state and key lifecycle
class EncryptionNotifier extends StateNotifier<EncryptionState> {
  final CryptoService _cryptoService;
  final ApiService _apiService;

  EncryptionNotifier(this._cryptoService, this._apiService)
      : super(EncryptionState()) {
    _init();
  }

  Future<void> _init() async {
    try {
      await _cryptoService.init();

      if (_cryptoService.hasKeys) {
        // Keys already exist, check prekey count from server
        await _checkPrekeyCount();
        state = state.copyWith(
          status: KeyRegistrationStatus.registered,
          deviceId: _cryptoService.deviceId,
        );
      } else {
        state = state.copyWith(status: KeyRegistrationStatus.notRegistered);
      }
    } catch (e) {
      state = state.copyWith(
        status: KeyRegistrationStatus.error,
        error: 'Failed to initialize encryption: $e',
      );
    }
  }

  /// Register encryption keys for this device
  Future<bool> registerKeys() async {
    if (state.status == KeyRegistrationStatus.registering) {
      return false;
    }

    state = state.copyWith(
      status: KeyRegistrationStatus.registering,
      error: null,
    );

    try {
      // Generate keys locally
      final keyData = await _cryptoService.generateKeys();

      // Register with server
      await _registerKeysWithServer(keyData);

      state = state.copyWith(
        status: KeyRegistrationStatus.registered,
        deviceId: keyData.deviceId,
        prekeyCount: keyData.preKeys.length,
        needsPreKeyUpload: false,
      );

      return true;
    } catch (e) {
      state = state.copyWith(
        status: KeyRegistrationStatus.error,
        error: 'Key registration failed: $e',
      );
      return false;
    }
  }

  /// Register keys with the backend server
  Future<void> _registerKeysWithServer(KeyRegistrationData keyData) async {
    // This would use the API service to call POST /api/keys/register
    // For now, we'll simulate the call
    // TODO: Add to ApiService:
    // await _apiService.registerKeys(keyData);
  }

  /// Check prekey count and upload more if needed
  Future<void> _checkPrekeyCount() async {
    try {
      // TODO: Call API to check prekey count
      // final response = await _apiService.getPreKeyCount(_cryptoService.deviceId);
      // if (response.needsUpload) {
      //   state = state.copyWith(needsPreKeyUpload: true, prekeyCount: response.count);
      // }
    } catch (e) {
      // Non-critical error
    }
  }

  /// Upload additional prekeys
  Future<void> uploadMorePreKeys({int count = 100}) async {
    try {
      // Generate new prekeys
      // TODO: Add method to crypto service for generating just prekeys
      // final prekeys = await _cryptoService.generatePreKeys(count);
      // await _apiService.uploadPreKeys(_cryptoService.deviceId, prekeys);

      state = state.copyWith(
        needsPreKeyUpload: false,
        prekeyCount: state.prekeyCount + count,
      );
    } catch (e) {
      // Log error but don't change state
    }
  }

  /// Encrypt a message for a recipient
  Future<List<EncryptedPayload>> encryptMessage(
    String plaintext,
    String recipientId,
  ) async {
    if (!state.isReady) {
      throw Exception('Encryption not ready');
    }

    // Get recipient's devices
    final devices = await _getRecipientDevices(recipientId);
    if (devices.isEmpty) {
      throw Exception('Recipient has no registered devices');
    }

    final payloads = <EncryptedPayload>[];

    for (final deviceId in devices) {
      // Ensure we have a session with this device
      await _ensureSession(recipientId, deviceId);

      // Encrypt for this device
      final payload = await _cryptoService.encrypt(
        plaintext,
        recipientId,
        deviceId,
      );
      payloads.add(payload);
    }

    return payloads;
  }

  /// Decrypt a message from a sender
  Future<String> decryptMessage(
    EncryptedPayload payload,
    String senderId,
    String senderDeviceId,
  ) async {
    if (!state.isReady) {
      throw Exception('Encryption not ready');
    }

    return await _cryptoService.decrypt(payload, senderId, senderDeviceId);
  }

  /// Ensure we have a session with a recipient's device
  Future<void> _ensureSession(String recipientId, String deviceId) async {
    // Try to encrypt - if no session exists, we need to fetch their prekey bundle
    try {
      // Check if session exists by trying a test operation
      // In production, we'd have a proper session check method
    } catch (e) {
      // Fetch prekey bundle and initialize session
      await _initializeSession(recipientId, deviceId);
    }
  }

  /// Initialize a session with a recipient's device
  Future<void> _initializeSession(String recipientId, String deviceId) async {
    // Fetch prekey bundle from server
    // TODO: Add to ApiService:
    // final bundle = await _apiService.getPreKeyBundle(recipientId, deviceId);
    // await _cryptoService.initializeSession(bundle);
  }

  /// Get list of recipient's devices
  Future<List<String>> _getRecipientDevices(String recipientId) async {
    // TODO: Call API to get devices
    // return await _apiService.getUserDevices(recipientId);
    return []; // Placeholder
  }

  /// Verify a contact's identity key
  Future<bool> verifyContact(
    String contactId,
    String expectedIdentityKey,
  ) async {
    // TODO: Implement verification
    // Compare displayed safety number with what contact shows

    // For now, just store as verified
    final updated = Map<String, String>.from(state.verifiedContacts);
    updated[contactId] = expectedIdentityKey;
    state = state.copyWith(verifiedContacts: updated);
    return true;
  }

  /// Check if a contact's identity key has changed
  bool hasIdentityKeyChanged(String contactId, String currentKey) {
    final storedKey = state.verifiedContacts[contactId];
    if (storedKey == null) return false;
    return storedKey != currentKey;
  }

  /// Get safety number for verification with a contact
  String getSafetyNumber(String contactId, String theirIdentityKey) {
    return _cryptoService.generateSafetyNumber(theirIdentityKey);
  }

  /// Clear all encryption data (on logout)
  Future<void> clear() async {
    await _cryptoService.clear();
    state = EncryptionState(status: KeyRegistrationStatus.notRegistered);
  }
}

/// Provider for encryption state management
final encryptionProvider =
    StateNotifierProvider<EncryptionNotifier, EncryptionState>((ref) {
  final cryptoService = ref.watch(cryptoServiceProvider);
  final apiService = ref.watch(apiServiceProvider);
  return EncryptionNotifier(cryptoService, apiService);
});

/// Provider for checking if encryption is ready
final isEncryptionReadyProvider = Provider<bool>((ref) {
  final state = ref.watch(encryptionProvider);
  return state.isReady;
});

/// Provider for device ID
final deviceIdProvider = Provider<String?>((ref) {
  final state = ref.watch(encryptionProvider);
  return state.deviceId;
});
