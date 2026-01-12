import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:timeago/timeago.dart' as timeago;
import '../providers/auth_provider.dart';
import '../providers/contacts_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/groups_provider.dart';
import '../models/contact.dart';
import 'chat_screen.dart';
import 'group_chat_screen.dart';
import 'create_group_screen.dart';
import 'search_screen.dart';

class ContactsScreen extends ConsumerStatefulWidget {
  const ContactsScreen({super.key});

  @override
  ConsumerState<ContactsScreen> createState() => _ContactsScreenState();
}

class _ContactsScreenState extends ConsumerState<ContactsScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);

    // Load data when screen opens
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(contactsProvider.notifier).loadContacts();
      ref.read(chatProvider.notifier).loadConversations();
      ref.read(groupsProvider.notifier).loadGroups();
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  void _showAddContactDialog() {
    final usernameController = TextEditingController();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Add Contact'),
        content: TextField(
          controller: usernameController,
          decoration: const InputDecoration(
            labelText: 'Username',
            hintText: 'Enter username to add',
          ),
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              final username = usernameController.text.trim();
              if (username.isNotEmpty) {
                ref.read(contactsProvider.notifier).addContact(username);
                Navigator.pop(context);
              }
            },
            child: const Text('Add'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final contactsState = ref.watch(contactsProvider);
    final chatState = ref.watch(chatProvider);
    final groupsState = ref.watch(groupsProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text('Hi, ${authState.user?.displayNameOrUsername ?? "User"}'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Chats'),
            Tab(text: 'Groups'),
            Tab(text: 'Contacts'),
          ],
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.search),
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => const SearchScreen(),
                ),
              );
            },
          ),
          IconButton(
            icon: const Icon(Icons.person_add_outlined),
            onPressed: _showAddContactDialog,
          ),
          IconButton(
            icon: const Icon(Icons.group_add_outlined),
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => const CreateGroupScreen(),
                ),
              );
            },
          ),
          PopupMenuButton(
            itemBuilder: (context) => [
              PopupMenuItem(
                child: const Text('Settings'),
                onTap: () {
                  // TODO: Navigate to settings
                },
              ),
              PopupMenuItem(
                child: const Text('Logout'),
                onTap: () {
                  ref.read(authProvider.notifier).logout();
                },
              ),
            ],
          ),
        ],
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          // Chats tab
          _buildConversationsList(chatState),

          // Groups tab
          _buildGroupsList(groupsState),

          // Contacts tab
          _buildContactsList(contactsState),
        ],
      ),
    );
  }

  Widget _buildConversationsList(ChatState chatState) {
    if (chatState.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (chatState.conversations.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 64,
              color: Colors.grey[400],
            ),
            const SizedBox(height: 16),
            Text(
              'No conversations yet',
              style: TextStyle(color: Colors.grey[600]),
            ),
            const SizedBox(height: 8),
            Text(
              'Add a contact to start chatting',
              style: TextStyle(color: Colors.grey[500], fontSize: 12),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => ref.read(chatProvider.notifier).loadConversations(),
      child: ListView.builder(
        itemCount: chatState.conversations.length,
        itemBuilder: (context, index) {
          final conversation = chatState.conversations[index];
          return ListTile(
            leading: CircleAvatar(
              child: Text(
                conversation.user.displayNameOrUsername[0].toUpperCase(),
              ),
            ),
            title: Text(conversation.user.displayNameOrUsername),
            subtitle: conversation.lastMessage != null
                ? Text(
                    conversation.lastMessage!.content ?? '[Media]',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  )
                : null,
            trailing: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                if (conversation.lastMessage != null)
                  Text(
                    timeago.format(conversation.lastMessage!.createdAt),
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey[600],
                    ),
                  ),
                if (conversation.unreadCount > 0)
                  Container(
                    margin: const EdgeInsets.only(top: 4),
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: Theme.of(context).primaryColor,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(
                      '${conversation.unreadCount}',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 12,
                      ),
                    ),
                  ),
              ],
            ),
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => ChatScreen(user: conversation.user),
                ),
              );
            },
          );
        },
      ),
    );
  }

  Widget _buildGroupsList(GroupsState groupsState) {
    if (groupsState.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (groupsState.groups.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.group_outlined,
              size: 64,
              color: Colors.grey[400],
            ),
            const SizedBox(height: 16),
            Text(
              'No groups yet',
              style: TextStyle(color: Colors.grey[600]),
            ),
            const SizedBox(height: 8),
            Text(
              'Create a group to start chatting',
              style: TextStyle(color: Colors.grey[500], fontSize: 12),
            ),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const CreateGroupScreen(),
                  ),
                );
              },
              icon: const Icon(Icons.group_add),
              label: const Text('Create Group'),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => ref.read(groupsProvider.notifier).loadGroups(),
      child: ListView.builder(
        itemCount: groupsState.groups.length,
        itemBuilder: (context, index) {
          final group = groupsState.groups[index];
          return ListTile(
            leading: CircleAvatar(
              backgroundColor: Theme.of(context).primaryColor,
              child: Text(
                group.name[0].toUpperCase(),
                style: const TextStyle(color: Colors.white),
              ),
            ),
            title: Text(group.name),
            subtitle: Text(
              '${group.memberCount} members',
              style: TextStyle(
                color: Colors.grey[600],
                fontSize: 12,
              ),
            ),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => GroupChatScreen(group: group),
                ),
              );
            },
          );
        },
      ),
    );
  }

  Widget _buildContactsList(ContactsState contactsState) {
    if (contactsState.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (contactsState.contacts.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.people_outline,
              size: 64,
              color: Colors.grey[400],
            ),
            const SizedBox(height: 16),
            Text(
              'No contacts yet',
              style: TextStyle(color: Colors.grey[600]),
            ),
            const SizedBox(height: 8),
            FilledButton.icon(
              onPressed: _showAddContactDialog,
              icon: const Icon(Icons.person_add),
              label: const Text('Add Contact'),
            ),
          ],
        ),
      );
    }

    // Sort contacts: online first
    final sortedContacts = [...contactsState.contacts]
      ..sort((a, b) {
        if (a.isOnline == b.isOnline) return 0;
        return a.isOnline ? -1 : 1;
      });

    return RefreshIndicator(
      onRefresh: () => ref.read(contactsProvider.notifier).loadContacts(),
      child: ListView.builder(
        itemCount: sortedContacts.length,
        itemBuilder: (context, index) {
          final contact = sortedContacts[index];
          return _buildContactTile(contact);
        },
      ),
    );
  }

  Widget _buildContactTile(Contact contact) {
    return ListTile(
      leading: Stack(
        children: [
          CircleAvatar(
            child: Text(contact.displayName[0].toUpperCase()),
          ),
          if (contact.isOnline)
            Positioned(
              right: 0,
              bottom: 0,
              child: Container(
                width: 12,
                height: 12,
                decoration: BoxDecoration(
                  color: Colors.green,
                  shape: BoxShape.circle,
                  border: Border.all(color: Colors.white, width: 2),
                ),
              ),
            ),
        ],
      ),
      title: Text(contact.displayName),
      subtitle: Text(
        contact.isOnline
            ? 'Online'
            : contact.user.lastSeen != null
                ? 'Last seen ${timeago.format(contact.user.lastSeen!)}'
                : 'Offline',
        style: TextStyle(
          color: contact.isOnline ? Colors.green : Colors.grey,
          fontSize: 12,
        ),
      ),
      trailing: IconButton(
        icon: const Icon(Icons.chat_bubble_outline),
        onPressed: () {
          Navigator.push(
            context,
            MaterialPageRoute(
              builder: (context) => ChatScreen(user: contact.user),
            ),
          );
        },
      ),
      onLongPress: () {
        showModalBottomSheet(
          context: context,
          builder: (context) => SafeArea(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  leading: const Icon(Icons.delete_outline, color: Colors.red),
                  title: const Text(
                    'Remove Contact',
                    style: TextStyle(color: Colors.red),
                  ),
                  onTap: () {
                    Navigator.pop(context);
                    ref.read(contactsProvider.notifier).removeContact(contact.id);
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
