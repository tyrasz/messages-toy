import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/contact.dart';
import '../services/api_service.dart';
import 'auth_provider.dart';

class ContactsState {
  final List<Contact> contacts;
  final bool isLoading;
  final String? error;

  ContactsState({
    this.contacts = const [],
    this.isLoading = false,
    this.error,
  });

  ContactsState copyWith({
    List<Contact>? contacts,
    bool? isLoading,
    String? error,
  }) {
    return ContactsState(
      contacts: contacts ?? this.contacts,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class ContactsNotifier extends StateNotifier<ContactsState> {
  final ApiService _apiService;

  ContactsNotifier(this._apiService) : super(ContactsState());

  Future<void> loadContacts() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final contacts = await _apiService.getContacts();
      state = state.copyWith(contacts: contacts, isLoading: false);
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to load contacts: ${e.toString()}',
        isLoading: false,
      );
    }
  }

  Future<void> addContact(String username, {String? nickname}) async {
    try {
      final contact = await _apiService.addContact(
        username: username,
        nickname: nickname,
      );
      state = state.copyWith(
        contacts: [...state.contacts, contact],
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to add contact: ${e.toString()}',
      );
    }
  }

  Future<void> removeContact(String contactId) async {
    try {
      await _apiService.removeContact(contactId);
      state = state.copyWith(
        contacts: state.contacts.where((c) => c.id != contactId).toList(),
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to remove contact: ${e.toString()}',
      );
    }
  }

  void updatePresence(String userId, bool online) {
    state = state.copyWith(
      contacts: state.contacts.map((contact) {
        if (contact.contactId == userId) {
          return Contact(
            id: contact.id,
            contactId: contact.contactId,
            nickname: contact.nickname,
            user: contact.user.copyWith(online: online),
            createdAt: contact.createdAt,
          );
        }
        return contact;
      }).toList(),
    );
  }
}

final contactsProvider = StateNotifierProvider<ContactsNotifier, ContactsState>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return ContactsNotifier(apiService);
});
