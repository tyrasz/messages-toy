import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../models/user.dart';
import '../models/contact.dart';
import '../models/message.dart';
import '../models/conversation.dart';
import '../models/group.dart';
import '../models/block.dart';
import '../models/search_result.dart';
import '../models/link_preview.dart';
import '../models/starred_message.dart';

class ApiService {
  static const String baseUrl = 'http://localhost:8080/api';

  final Dio _dio;
  final FlutterSecureStorage _storage;

  String? _accessToken;
  String? _refreshToken;
  User? _currentUser;

  ApiService()
      : _dio = Dio(BaseOptions(
          baseUrl: baseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
        )),
        _storage = const FlutterSecureStorage() {
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        if (_accessToken != null) {
          options.headers['Authorization'] = 'Bearer $_accessToken';
        }
        return handler.next(options);
      },
      onError: (error, handler) async {
        if (error.response?.statusCode == 401 && _refreshToken != null) {
          // Try to refresh token
          try {
            await _refreshAccessToken();
            // Retry the request
            final opts = error.requestOptions;
            opts.headers['Authorization'] = 'Bearer $_accessToken';
            final response = await _dio.fetch(opts);
            return handler.resolve(response);
          } catch (e) {
            // Refresh failed, logout
            await logout();
          }
        }
        return handler.next(error);
      },
    ));
  }

  Future<void> init() async {
    _accessToken = await _storage.read(key: 'access_token');
    _refreshToken = await _storage.read(key: 'refresh_token');
    final userJson = await _storage.read(key: 'user');
    if (userJson != null) {
      // Parse user from stored JSON
    }
  }

  bool get isAuthenticated => _accessToken != null;
  User? get currentUser => _currentUser;
  String? get accessToken => _accessToken;

  Future<void> _refreshAccessToken() async {
    final response = await _dio.post('/auth/refresh', data: {
      'refresh_token': _refreshToken,
    });

    _accessToken = response.data['access_token'];
    _refreshToken = response.data['refresh_token'];

    await _storage.write(key: 'access_token', value: _accessToken);
    await _storage.write(key: 'refresh_token', value: _refreshToken);
  }

  // Auth endpoints

  Future<User> register({
    required String username,
    required String password,
    String? phone,
    String? displayName,
  }) async {
    final response = await _dio.post('/auth/register', data: {
      'username': username,
      'password': password,
      if (phone != null) 'phone': phone,
      if (displayName != null) 'display_name': displayName,
    });

    await _handleAuthResponse(response.data);
    return _currentUser!;
  }

  Future<User> login({
    required String username,
    required String password,
  }) async {
    final response = await _dio.post('/auth/login', data: {
      'username': username,
      'password': password,
    });

    await _handleAuthResponse(response.data);
    return _currentUser!;
  }

  Future<void> _handleAuthResponse(Map<String, dynamic> data) async {
    _accessToken = data['access_token'];
    _refreshToken = data['refresh_token'];
    _currentUser = User.fromJson(data['user']);

    await _storage.write(key: 'access_token', value: _accessToken);
    await _storage.write(key: 'refresh_token', value: _refreshToken);
  }

  Future<void> logout() async {
    _accessToken = null;
    _refreshToken = null;
    _currentUser = null;
    await _storage.deleteAll();
  }

  // Contacts endpoints

  Future<List<Contact>> getContacts() async {
    final response = await _dio.get('/contacts');
    final contacts = (response.data['contacts'] as List)
        .map((json) => Contact.fromJson(json))
        .toList();
    return contacts;
  }

  Future<Contact> addContact({
    required String username,
    String? nickname,
  }) async {
    final response = await _dio.post('/contacts', data: {
      'username': username,
      if (nickname != null) 'nickname': nickname,
    });
    return Contact.fromJson(response.data);
  }

  Future<void> removeContact(String contactId) async {
    await _dio.delete('/contacts/$contactId');
  }

  // Messages endpoints

  Future<List<Conversation>> getConversations() async {
    final response = await _dio.get('/messages/conversations');
    final conversations = (response.data['conversations'] as List)
        .map((json) => Conversation.fromJson(json))
        .toList();
    return conversations;
  }

  Future<List<Message>> getMessages(String userId, {int limit = 50, int offset = 0}) async {
    final response = await _dio.get('/messages/$userId', queryParameters: {
      'limit': limit,
      'offset': offset,
    });
    final messages = (response.data['messages'] as List)
        .map((json) => Message.fromJson(json))
        .toList();
    return messages;
  }

  Future<List<SearchResult>> searchMessages(String query, {int limit = 20, int offset = 0}) async {
    final response = await _dio.get('/messages/search', queryParameters: {
      'q': query,
      'limit': limit,
      'offset': offset,
    });
    final results = (response.data['results'] as List)
        .map((json) => SearchResult.fromJson(json))
        .toList();
    return results;
  }

  // Media endpoints

  Future<Map<String, dynamic>> uploadMedia(String filePath) async {
    final formData = FormData.fromMap({
      'file': await MultipartFile.fromFile(filePath),
    });

    final response = await _dio.post('/media/upload', data: formData);
    return response.data;
  }

  // Group endpoints

  Future<List<Group>> getGroups() async {
    final response = await _dio.get('/groups');
    final groups = (response.data['groups'] as List)
        .map((json) => Group.fromJson(json))
        .toList();
    return groups;
  }

  Future<Group> createGroup({
    required String name,
    String? description,
    List<String>? memberIds,
  }) async {
    final response = await _dio.post('/groups', data: {
      'name': name,
      if (description != null) 'description': description,
      if (memberIds != null) 'member_ids': memberIds,
    });
    return Group.fromJson(response.data);
  }

  Future<Group> getGroup(String groupId) async {
    final response = await _dio.get('/groups/$groupId');
    return Group.fromJson(response.data);
  }

  Future<void> addGroupMember(String groupId, String userId) async {
    await _dio.post('/groups/$groupId/members', data: {
      'user_id': userId,
    });
  }

  Future<void> removeGroupMember(String groupId, String userId) async {
    await _dio.delete('/groups/$groupId/members/$userId');
  }

  Future<void> leaveGroup(String groupId) async {
    await _dio.post('/groups/$groupId/leave');
  }

  Future<List<Message>> getGroupMessages(String groupId, {int limit = 100}) async {
    final response = await _dio.get('/groups/$groupId/messages', queryParameters: {
      'limit': limit,
    });
    final messages = (response.data['messages'] as List)
        .map((json) => Message.fromJson(json))
        .toList();
    return messages;
  }

  // Block endpoints

  Future<List<BlockedUser>> getBlocks() async {
    final response = await _dio.get('/blocks');
    final blocks = (response.data['blocked'] as List)
        .map((json) => BlockedUser.fromJson(json))
        .toList();
    return blocks;
  }

  Future<BlockedUser> blockUser(String userId) async {
    final response = await _dio.post('/blocks/$userId');
    return BlockedUser.fromJson(response.data);
  }

  Future<void> unblockUser(String userId) async {
    await _dio.delete('/blocks/$userId');
  }

  Future<bool> isBlocked(String userId) async {
    final response = await _dio.get('/blocks/$userId');
    return response.data['blocked'] as bool;
  }

  // Link previews
  Future<LinkPreview?> fetchLinkPreview(String url) async {
    try {
      final response = await _dio.post('/links/preview', data: {'url': url});
      final previewData = response.data['preview'] as Map<String, dynamic>?;
      if (previewData != null) {
        return LinkPreview.fromJson(previewData);
      }
      return null;
    } catch (e) {
      return null;
    }
  }

  // Starred messages
  Future<List<StarredMessage>> getStarredMessages({int limit = 50, int offset = 0}) async {
    final response = await _dio.get('/starred', queryParameters: {
      'limit': limit,
      'offset': offset,
    });
    final starred = (response.data['starred'] as List?)
            ?.map((json) => StarredMessage.fromJson(json as Map<String, dynamic>))
            .toList() ??
        [];
    return starred;
  }

  Future<bool> starMessage(String messageId) async {
    final response = await _dio.post('/starred/$messageId');
    return response.data['starred'] as bool;
  }

  Future<bool> unstarMessage(String messageId) async {
    await _dio.delete('/starred/$messageId');
    return false;
  }

  Future<bool> isMessageStarred(String messageId) async {
    final response = await _dio.get('/starred/$messageId');
    return response.data['starred'] as bool;
  }

  // Conversation settings (disappearing messages)
  Future<int> getDisappearingTimer({String? otherUserId, String? groupId}) async {
    final params = <String, dynamic>{};
    if (otherUserId != null) params['other_user_id'] = otherUserId;
    if (groupId != null) params['group_id'] = groupId;

    final response = await _dio.get('/settings/conversation', queryParameters: params);
    final settings = response.data['settings'] as Map<String, dynamic>?;
    return settings?['disappearing_seconds'] as int? ?? 0;
  }

  Future<void> setDisappearingTimer({
    String? otherUserId,
    String? groupId,
    required int seconds,
  }) async {
    await _dio.post('/settings/disappearing', data: {
      if (otherUserId != null) 'other_user_id': otherUserId,
      if (groupId != null) 'group_id': groupId,
      'seconds': seconds,
    });
  }

  Future<void> muteConversation({
    String? otherUserId,
    String? groupId,
    required int hours,
  }) async {
    await _dio.post('/settings/mute', data: {
      if (otherUserId != null) 'other_user_id': otherUserId,
      if (groupId != null) 'group_id': groupId,
      'hours': hours,
    });
  }
}
