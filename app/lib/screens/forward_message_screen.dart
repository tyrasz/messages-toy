import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/message.dart';
import '../models/contact.dart';
import '../models/group.dart';
import '../providers/contacts_provider.dart';
import '../providers/groups_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/auth_provider.dart';

class ForwardMessageScreen extends ConsumerStatefulWidget {
  final Message message;
  final String originalSenderName;

  const ForwardMessageScreen({
    super.key,
    required this.message,
    required this.originalSenderName,
  });

  @override
  ConsumerState<ForwardMessageScreen> createState() => _ForwardMessageScreenState();
}

class _ForwardMessageScreenState extends ConsumerState<ForwardMessageScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  final Set<String> _selectedContacts = {};
  final Set<String> _selectedGroups = {};

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);

    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(contactsProvider.notifier).loadContacts();
      ref.read(groupsProvider.notifier).loadGroups();
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  void _toggleContact(String contactId) {
    setState(() {
      if (_selectedContacts.contains(contactId)) {
        _selectedContacts.remove(contactId);
      } else {
        _selectedContacts.add(contactId);
      }
    });
  }

  void _toggleGroup(String groupId) {
    setState(() {
      if (_selectedGroups.contains(groupId)) {
        _selectedGroups.remove(groupId);
      } else {
        _selectedGroups.add(groupId);
      }
    });
  }

  void _forwardMessage() {
    final forwardedFrom = widget.message.isForwarded
        ? widget.message.forwardedFrom!
        : widget.originalSenderName;

    // Forward to contacts
    for (final contactId in _selectedContacts) {
      ref.read(chatProvider.notifier).forwardMessage(
            widget.message,
            contactId,
            forwardedFrom,
          );
    }

    // Forward to groups via WebSocket
    final webSocketService = ref.read(webSocketServiceProvider);
    for (final groupId in _selectedGroups) {
      webSocketService.sendMessage(
        groupId: groupId,
        content: widget.message.content,
        mediaId: widget.message.mediaId,
        forwardedFrom: forwardedFrom,
      );
    }

    final count = _selectedContacts.length + _selectedGroups.length;
    Navigator.pop(context);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Message forwarded to $count ${count == 1 ? 'chat' : 'chats'}'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final contactsState = ref.watch(contactsProvider);
    final groupsState = ref.watch(groupsProvider);
    final totalSelected = _selectedContacts.length + _selectedGroups.length;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Forward to'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Contacts'),
            Tab(text: 'Groups'),
          ],
        ),
      ),
      body: Column(
        children: [
          // Message preview
          Container(
            padding: const EdgeInsets.all(12),
            color: Colors.grey[100],
            child: Row(
              children: [
                Icon(Icons.reply, color: Colors.grey[600], size: 20),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (widget.message.isForwarded)
                        Text(
                          'Forwarded from ${widget.message.forwardedFrom}',
                          style: TextStyle(
                            fontSize: 11,
                            color: Colors.grey[600],
                            fontStyle: FontStyle.italic,
                          ),
                        ),
                      Text(
                        widget.message.content ?? '[Media]',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 14),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          // Tabs content
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                // Contacts tab
                _buildContactsList(contactsState),

                // Groups tab
                _buildGroupsList(groupsState),
              ],
            ),
          ),
        ],
      ),
      floatingActionButton: totalSelected > 0
          ? FloatingActionButton.extended(
              onPressed: _forwardMessage,
              icon: const Icon(Icons.send),
              label: Text('Forward ($totalSelected)'),
            )
          : null,
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
            Icon(Icons.people_outline, size: 48, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'No contacts yet',
              style: TextStyle(color: Colors.grey[600]),
            ),
          ],
        ),
      );
    }

    return ListView.builder(
      itemCount: contactsState.contacts.length,
      itemBuilder: (context, index) {
        final contact = contactsState.contacts[index];
        final isSelected = _selectedContacts.contains(contact.user.id);

        return ListTile(
          leading: Stack(
            children: [
              CircleAvatar(
                child: Text(contact.displayName[0].toUpperCase()),
              ),
              if (isSelected)
                Positioned(
                  right: 0,
                  bottom: 0,
                  child: Container(
                    width: 18,
                    height: 18,
                    decoration: BoxDecoration(
                      color: Theme.of(context).primaryColor,
                      shape: BoxShape.circle,
                      border: Border.all(color: Colors.white, width: 2),
                    ),
                    child: const Icon(
                      Icons.check,
                      size: 12,
                      color: Colors.white,
                    ),
                  ),
                ),
            ],
          ),
          title: Text(contact.displayName),
          subtitle: Text(
            contact.isOnline ? 'Online' : 'Offline',
            style: TextStyle(
              fontSize: 12,
              color: contact.isOnline ? Colors.green : Colors.grey,
            ),
          ),
          onTap: () => _toggleContact(contact.user.id),
          selected: isSelected,
          selectedTileColor: Theme.of(context).primaryColor.withOpacity(0.1),
        );
      },
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
            Icon(Icons.group_outlined, size: 48, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'No groups yet',
              style: TextStyle(color: Colors.grey[600]),
            ),
          ],
        ),
      );
    }

    return ListView.builder(
      itemCount: groupsState.groups.length,
      itemBuilder: (context, index) {
        final group = groupsState.groups[index];
        final isSelected = _selectedGroups.contains(group.id);

        return ListTile(
          leading: Stack(
            children: [
              CircleAvatar(
                backgroundColor: Theme.of(context).primaryColor,
                child: Text(
                  group.name[0].toUpperCase(),
                  style: const TextStyle(color: Colors.white),
                ),
              ),
              if (isSelected)
                Positioned(
                  right: 0,
                  bottom: 0,
                  child: Container(
                    width: 18,
                    height: 18,
                    decoration: BoxDecoration(
                      color: Theme.of(context).primaryColor,
                      shape: BoxShape.circle,
                      border: Border.all(color: Colors.white, width: 2),
                    ),
                    child: const Icon(
                      Icons.check,
                      size: 12,
                      color: Colors.white,
                    ),
                  ),
                ),
            ],
          ),
          title: Text(group.name),
          subtitle: Text(
            '${group.memberCount} members',
            style: TextStyle(fontSize: 12, color: Colors.grey[600]),
          ),
          onTap: () => _toggleGroup(group.id),
          selected: isSelected,
          selectedTileColor: Theme.of(context).primaryColor.withOpacity(0.1),
        );
      },
    );
  }
}
