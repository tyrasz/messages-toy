import 'dart:convert';
import 'dart:typed_data';
import 'package:cryptography/cryptography.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:uuid/uuid.dart';

/// E2EE Crypto Service implementing Signal Protocol concepts
/// - X25519 key exchange
/// - ChaCha20-Poly1305 AEAD encryption
/// - Double Ratchet-like session management
class CryptoService {
  static const String _identityKeyKey = 'e2ee_identity_key';
  static const String _signedPreKeyKey = 'e2ee_signed_prekey';
  static const String _deviceIdKey = 'e2ee_device_id';
  static const String _registrationIdKey = 'e2ee_registration_id';

  final FlutterSecureStorage _secureStorage;
  final _uuid = const Uuid();

  late Box<Map> _sessionsBox;
  late Box<Map> _preKeysBox;

  bool _initialized = false;
  String? _deviceId;
  int? _registrationId;

  // Key pairs (private stored securely)
  SimpleKeyPair? _identityKeyPair;
  SimpleKeyPair? _signedPreKeyPair;
  final List<SimpleKeyPair> _preKeys = [];

  CryptoService() : _secureStorage = const FlutterSecureStorage();

  /// Initialize the crypto service
  Future<void> init() async {
    if (_initialized) return;

    await Hive.initFlutter();
    _sessionsBox = await Hive.openBox<Map>('e2ee_sessions');
    _preKeysBox = await Hive.openBox<Map>('e2ee_prekeys');

    // Load device ID
    _deviceId = await _secureStorage.read(key: _deviceIdKey);
    if (_deviceId == null) {
      _deviceId = _uuid.v4();
      await _secureStorage.write(key: _deviceIdKey, value: _deviceId);
    }

    // Load registration ID
    final regIdStr = await _secureStorage.read(key: _registrationIdKey);
    if (regIdStr != null) {
      _registrationId = int.parse(regIdStr);
    }

    // Load existing keys if present
    await _loadKeys();

    _initialized = true;
  }

  String get deviceId => _deviceId ?? '';
  int get registrationId => _registrationId ?? 0;
  bool get hasKeys => _identityKeyPair != null && _signedPreKeyPair != null;

  /// Generate new identity keys (called on first registration)
  Future<KeyRegistrationData> generateKeys({int preKeyCount = 100}) async {
    final algorithm = X25519();

    // Generate identity key pair
    _identityKeyPair = await algorithm.newKeyPair();

    // Generate signed prekey
    _signedPreKeyPair = await algorithm.newKeyPair();

    // Generate registration ID
    _registrationId = DateTime.now().millisecondsSinceEpoch % 0xFFFFFF;

    // Generate one-time prekeys
    _preKeys.clear();
    final preKeysList = <PreKeyData>[];
    for (int i = 0; i < preKeyCount; i++) {
      final keyPair = await algorithm.newKeyPair();
      _preKeys.add(keyPair);

      final publicKey = await keyPair.extractPublicKey();
      preKeysList.add(PreKeyData(
        keyId: i,
        publicKey: base64Encode(publicKey.bytes),
      ));

      // Store private key
      final privateBytes = await keyPair.extractPrivateKeyBytes();
      await _preKeysBox.put('prekey_$i', {
        'key_id': i,
        'private_key': base64Encode(privateBytes),
      });
    }

    // Store keys securely
    await _saveKeys();

    // Get public keys
    final identityPublic = await _identityKeyPair!.extractPublicKey();
    final signedPreKeyPublic = await _signedPreKeyPair!.extractPublicKey();

    // Sign the signed prekey with identity key
    final signature = await _signKey(signedPreKeyPublic.bytes);

    return KeyRegistrationData(
      deviceId: _deviceId!,
      registrationId: _registrationId!,
      identityKey: base64Encode(identityPublic.bytes),
      signedPreKey: SignedPreKeyData(
        keyId: 0,
        publicKey: base64Encode(signedPreKeyPublic.bytes),
        signature: base64Encode(signature),
      ),
      preKeys: preKeysList,
    );
  }

  /// Sign a key using our identity key
  Future<Uint8List> _signKey(List<int> data) async {
    // In production, use Ed25519 for signing
    // For now, we use a simple HMAC-based approach
    final algorithm = Hmac.sha256();
    final privateBytes = await _identityKeyPair!.extractPrivateKeyBytes();
    final secretKey = SecretKey(privateBytes);
    final mac = await algorithm.calculateMac(data, secretKey: secretKey);
    return Uint8List.fromList(mac.bytes);
  }

  /// Load keys from secure storage
  Future<void> _loadKeys() async {
    try {
      final identityPrivate = await _secureStorage.read(key: _identityKeyKey);
      final signedPreKeyPrivate = await _secureStorage.read(key: _signedPreKeyKey);

      if (identityPrivate != null && signedPreKeyPrivate != null) {
        final algorithm = X25519();

        _identityKeyPair = await algorithm.newKeyPairFromSeed(
          base64Decode(identityPrivate),
        );
        _signedPreKeyPair = await algorithm.newKeyPairFromSeed(
          base64Decode(signedPreKeyPrivate),
        );
      }
    } catch (e) {
      // Keys not found or corrupted
      _identityKeyPair = null;
      _signedPreKeyPair = null;
    }
  }

  /// Save keys to secure storage
  Future<void> _saveKeys() async {
    if (_identityKeyPair != null) {
      final privateBytes = await _identityKeyPair!.extractPrivateKeyBytes();
      await _secureStorage.write(
        key: _identityKeyKey,
        value: base64Encode(privateBytes),
      );
    }

    if (_signedPreKeyPair != null) {
      final privateBytes = await _signedPreKeyPair!.extractPrivateKeyBytes();
      await _secureStorage.write(
        key: _signedPreKeyKey,
        value: base64Encode(privateBytes),
      );
    }

    if (_registrationId != null) {
      await _secureStorage.write(
        key: _registrationIdKey,
        value: _registrationId.toString(),
      );
    }
  }

  /// Initialize a session with a recipient using their prekey bundle
  Future<SessionState> initializeSession(PreKeyBundle bundle) async {
    final algorithm = X25519();
    final cipher = Chacha20.poly1305Aead();

    // Decode recipient's keys
    final theirIdentityKey = SimplePublicKey(
      base64Decode(bundle.identityKey),
      type: KeyPairType.x25519,
    );
    final theirSignedPreKey = SimplePublicKey(
      base64Decode(bundle.signedPreKey.publicKey),
      type: KeyPairType.x25519,
    );

    // Perform X3DH key agreement
    // DH1: Our identity key with their signed prekey
    final dh1 = await algorithm.sharedSecretKey(
      keyPair: _identityKeyPair!,
      remotePublicKey: theirSignedPreKey,
    );

    // DH2: Our ephemeral key with their identity key
    final ephemeralKeyPair = await algorithm.newKeyPair();
    final dh2 = await algorithm.sharedSecretKey(
      keyPair: ephemeralKeyPair,
      remotePublicKey: theirIdentityKey,
    );

    // DH3: Our ephemeral key with their signed prekey
    final dh3 = await algorithm.sharedSecretKey(
      keyPair: ephemeralKeyPair,
      remotePublicKey: theirSignedPreKey,
    );

    // Combine secrets to derive root key
    final combinedSecret = [
      ...await dh1.extractBytes(),
      ...await dh2.extractBytes(),
      ...await dh3.extractBytes(),
    ];

    // DH4: If they have a one-time prekey
    if (bundle.preKey != null) {
      final theirPreKey = SimplePublicKey(
        base64Decode(bundle.preKey!.publicKey),
        type: KeyPairType.x25519,
      );
      final dh4 = await algorithm.sharedSecretKey(
        keyPair: ephemeralKeyPair,
        remotePublicKey: theirPreKey,
      );
      combinedSecret.addAll(await dh4.extractBytes());
    }

    // Derive root key using HKDF
    final hkdf = Hkdf(hmac: Hmac.sha256(), outputLength: 64);
    final derived = await hkdf.deriveKey(
      secretKey: SecretKey(combinedSecret),
      nonce: Uint8List(0),
      info: utf8.encode('Signal_X3DH'),
    );
    final derivedBytes = await derived.extractBytes();

    // Split into root key and chain key
    final rootKey = Uint8List.fromList(derivedBytes.sublist(0, 32));
    final chainKey = Uint8List.fromList(derivedBytes.sublist(32, 64));

    // Get our ephemeral public key to send
    final ephemeralPublic = await ephemeralKeyPair.extractPublicKey();

    final session = SessionState(
      recipientId: bundle.userId,
      recipientDeviceId: bundle.deviceId,
      rootKey: base64Encode(rootKey),
      sendingChainKey: base64Encode(chainKey),
      receivingChainKey: '',
      sendingCounter: 0,
      receivingCounter: 0,
      ephemeralPublicKey: base64Encode(ephemeralPublic.bytes),
      preKeyId: bundle.preKey?.keyId,
    );

    // Store session
    await _saveSession(session);

    return session;
  }

  /// Encrypt a message for a recipient
  Future<EncryptedPayload> encrypt(
    String plaintext,
    String recipientId,
    String recipientDeviceId,
  ) async {
    final session = await _loadSession(recipientId, recipientDeviceId);
    if (session == null) {
      throw Exception('No session found for $recipientId:$recipientDeviceId');
    }

    final cipher = Chacha20.poly1305Aead();

    // Derive message key from chain key
    final chainKey = base64Decode(session.sendingChainKey);
    final messageKey = await _deriveMessageKey(chainKey);

    // Generate nonce
    final nonce = Uint8List(12);
    for (int i = 0; i < 12; i++) {
      nonce[i] = (session.sendingCounter >> (i * 8)) & 0xFF;
    }

    // Encrypt
    final secretBox = await cipher.encrypt(
      utf8.encode(plaintext),
      secretKey: SecretKey(messageKey),
      nonce: nonce,
    );

    // Ratchet chain key
    final newChainKey = await _ratchetChainKey(chainKey);

    // Update session
    final updatedSession = session.copyWith(
      sendingChainKey: base64Encode(newChainKey),
      sendingCounter: session.sendingCounter + 1,
    );
    await _saveSession(updatedSession);

    // Message type: 1 = PreKeyMessage (first message), 2 = regular message
    final isPreKeyMessage = session.preKeyId != null && session.sendingCounter == 1;

    return EncryptedPayload(
      recipientDeviceId: recipientDeviceId,
      cipherText: base64Encode([
        ...secretBox.nonce,
        ...secretBox.cipherText,
        ...secretBox.mac.bytes,
      ]),
      messageType: isPreKeyMessage ? 1 : 2,
    );
  }

  /// Decrypt a message from a sender
  Future<String> decrypt(
    EncryptedPayload payload,
    String senderId,
    String senderDeviceId,
  ) async {
    var session = await _loadSession(senderId, senderDeviceId);

    // If PreKeyMessage and no session, we need to create one
    if (session == null && payload.messageType == 1) {
      // In production, this would extract the prekey bundle from the message
      // and initialize the session
      throw Exception('Session initialization from PreKeyMessage not implemented');
    }

    if (session == null) {
      throw Exception('No session found for $senderId:$senderDeviceId');
    }

    final cipher = Chacha20.poly1305Aead();
    final cipherBytes = base64Decode(payload.cipherText);

    // Extract nonce, ciphertext, and MAC
    final nonce = cipherBytes.sublist(0, 12);
    final cipherText = cipherBytes.sublist(12, cipherBytes.length - 16);
    final mac = Mac(cipherBytes.sublist(cipherBytes.length - 16));

    // Derive message key from receiving chain key
    if (session.receivingChainKey.isEmpty) {
      // First message - use sending chain key for now
      // In production, proper ratchet initialization needed
      session = session.copyWith(receivingChainKey: session.sendingChainKey);
    }

    final chainKey = base64Decode(session.receivingChainKey);
    final messageKey = await _deriveMessageKey(chainKey);

    // Decrypt
    final secretBox = SecretBox(cipherText, nonce: nonce, mac: mac);
    final plainBytes = await cipher.decrypt(
      secretBox,
      secretKey: SecretKey(messageKey),
    );

    // Ratchet receiving chain key
    final newChainKey = await _ratchetChainKey(chainKey);
    final updatedSession = session.copyWith(
      receivingChainKey: base64Encode(newChainKey),
      receivingCounter: session.receivingCounter + 1,
    );
    await _saveSession(updatedSession);

    return utf8.decode(plainBytes);
  }

  /// Derive message key from chain key
  Future<Uint8List> _deriveMessageKey(Uint8List chainKey) async {
    final hmac = Hmac.sha256();
    final mac = await hmac.calculateMac(
      [0x01], // Message key constant
      secretKey: SecretKey(chainKey),
    );
    return Uint8List.fromList(mac.bytes);
  }

  /// Ratchet chain key forward
  Future<Uint8List> _ratchetChainKey(Uint8List chainKey) async {
    final hmac = Hmac.sha256();
    final mac = await hmac.calculateMac(
      [0x02], // Chain key constant
      secretKey: SecretKey(chainKey),
    );
    return Uint8List.fromList(mac.bytes);
  }

  /// Load a session from storage
  Future<SessionState?> _loadSession(String recipientId, String deviceId) async {
    final key = 'session_${recipientId}_$deviceId';
    final data = _sessionsBox.get(key);
    if (data == null) return null;
    return SessionState.fromMap(Map<String, dynamic>.from(data));
  }

  /// Save a session to storage
  Future<void> _saveSession(SessionState session) async {
    final key = 'session_${session.recipientId}_${session.recipientDeviceId}';
    await _sessionsBox.put(key, session.toMap());
  }

  /// Generate safety number for verification
  String generateSafetyNumber(String theirIdentityKey) {
    if (_identityKeyPair == null) return '';

    // Combine both identity keys and hash
    // Return first 60 digits formatted as groups of 5
    // This is a simplified version
    final combined = '$theirIdentityKey';
    final hash = combined.hashCode.abs().toString().padLeft(60, '0');
    final groups = <String>[];
    for (int i = 0; i < 60; i += 5) {
      groups.add(hash.substring(i, i + 5));
    }
    return groups.join(' ');
  }

  /// Clear all encryption data (on logout)
  Future<void> clear() async {
    await _secureStorage.delete(key: _identityKeyKey);
    await _secureStorage.delete(key: _signedPreKeyKey);
    await _secureStorage.delete(key: _registrationIdKey);
    await _sessionsBox.clear();
    await _preKeysBox.clear();
    _identityKeyPair = null;
    _signedPreKeyPair = null;
    _preKeys.clear();
    _initialized = false;
  }
}

/// Data classes

class KeyRegistrationData {
  final String deviceId;
  final int registrationId;
  final String identityKey;
  final SignedPreKeyData signedPreKey;
  final List<PreKeyData> preKeys;

  KeyRegistrationData({
    required this.deviceId,
    required this.registrationId,
    required this.identityKey,
    required this.signedPreKey,
    required this.preKeys,
  });

  Map<String, dynamic> toJson() => {
        'device_id': deviceId,
        'registration_id': registrationId,
        'identity_key': identityKey,
        'signed_prekey': signedPreKey.toJson(),
        'prekeys': preKeys.map((p) => p.toJson()).toList(),
      };
}

class SignedPreKeyData {
  final int keyId;
  final String publicKey;
  final String signature;

  SignedPreKeyData({
    required this.keyId,
    required this.publicKey,
    required this.signature,
  });

  Map<String, dynamic> toJson() => {
        'key_id': keyId,
        'public_key': publicKey,
        'signature': signature,
      };

  factory SignedPreKeyData.fromJson(Map<String, dynamic> json) => SignedPreKeyData(
        keyId: json['key_id'] as int,
        publicKey: json['public_key'] as String,
        signature: json['signature'] as String,
      );
}

class PreKeyData {
  final int keyId;
  final String publicKey;

  PreKeyData({
    required this.keyId,
    required this.publicKey,
  });

  Map<String, dynamic> toJson() => {
        'key_id': keyId,
        'public_key': publicKey,
      };

  factory PreKeyData.fromJson(Map<String, dynamic> json) => PreKeyData(
        keyId: json['key_id'] as int,
        publicKey: json['public_key'] as String,
      );
}

class PreKeyBundle {
  final String userId;
  final String deviceId;
  final String identityKey;
  final SignedPreKeyData signedPreKey;
  final PreKeyData? preKey;

  PreKeyBundle({
    required this.userId,
    required this.deviceId,
    required this.identityKey,
    required this.signedPreKey,
    this.preKey,
  });

  factory PreKeyBundle.fromJson(Map<String, dynamic> json) => PreKeyBundle(
        userId: json['identity_key']['user_id'] as String,
        deviceId: json['identity_key']['device_id'] as String,
        identityKey: json['identity_key']['public_key'] as String,
        signedPreKey: SignedPreKeyData.fromJson(json['signed_prekey']),
        preKey: json['prekey'] != null ? PreKeyData.fromJson(json['prekey']) : null,
      );
}

class SessionState {
  final String recipientId;
  final String recipientDeviceId;
  final String rootKey;
  final String sendingChainKey;
  final String receivingChainKey;
  final int sendingCounter;
  final int receivingCounter;
  final String ephemeralPublicKey;
  final int? preKeyId;

  SessionState({
    required this.recipientId,
    required this.recipientDeviceId,
    required this.rootKey,
    required this.sendingChainKey,
    required this.receivingChainKey,
    required this.sendingCounter,
    required this.receivingCounter,
    required this.ephemeralPublicKey,
    this.preKeyId,
  });

  SessionState copyWith({
    String? rootKey,
    String? sendingChainKey,
    String? receivingChainKey,
    int? sendingCounter,
    int? receivingCounter,
  }) =>
      SessionState(
        recipientId: recipientId,
        recipientDeviceId: recipientDeviceId,
        rootKey: rootKey ?? this.rootKey,
        sendingChainKey: sendingChainKey ?? this.sendingChainKey,
        receivingChainKey: receivingChainKey ?? this.receivingChainKey,
        sendingCounter: sendingCounter ?? this.sendingCounter,
        receivingCounter: receivingCounter ?? this.receivingCounter,
        ephemeralPublicKey: ephemeralPublicKey,
        preKeyId: preKeyId,
      );

  Map<String, dynamic> toMap() => {
        'recipient_id': recipientId,
        'recipient_device_id': recipientDeviceId,
        'root_key': rootKey,
        'sending_chain_key': sendingChainKey,
        'receiving_chain_key': receivingChainKey,
        'sending_counter': sendingCounter,
        'receiving_counter': receivingCounter,
        'ephemeral_public_key': ephemeralPublicKey,
        'prekey_id': preKeyId,
      };

  factory SessionState.fromMap(Map<String, dynamic> map) => SessionState(
        recipientId: map['recipient_id'] as String,
        recipientDeviceId: map['recipient_device_id'] as String,
        rootKey: map['root_key'] as String,
        sendingChainKey: map['sending_chain_key'] as String,
        receivingChainKey: map['receiving_chain_key'] as String,
        sendingCounter: map['sending_counter'] as int,
        receivingCounter: map['receiving_counter'] as int,
        ephemeralPublicKey: map['ephemeral_public_key'] as String,
        preKeyId: map['prekey_id'] as int?,
      );
}

class EncryptedPayload {
  final String recipientDeviceId;
  final String cipherText;
  final int messageType;

  EncryptedPayload({
    required this.recipientDeviceId,
    required this.cipherText,
    required this.messageType,
  });

  Map<String, dynamic> toJson() => {
        'recipient_device_id': recipientDeviceId,
        'cipher_text': cipherText,
        'message_type': messageType,
      };

  factory EncryptedPayload.fromJson(Map<String, dynamic> json) => EncryptedPayload(
        recipientDeviceId: json['recipient_device_id'] as String,
        cipherText: json['cipher_text'] as String,
        messageType: json['message_type'] as int,
      );
}
