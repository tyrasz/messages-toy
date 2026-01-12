import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/contact.dart';
import '../providers/contacts_provider.dart';
import '../providers/groups_provider.dart';
import 'group_chat_screen.dart';

class CreateGroupScreen extends ConsumerStatefulWidget {
  const CreateGroupScreen({super.key});

  @override
  ConsumerState<CreateGroupScreen> createState() => _CreateGroupScreenState();
}

class _CreateGroupScreenState extends ConsumerState<CreateGroupScreen> {
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final Set<String> _selectedMemberIds = {};
  bool _isCreating = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(contactsProvider.notifier).loadContacts();
    });
  }

  @override
  void dispose() {
    _nameController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  Future<void> _createGroup() async {
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please enter a group name')),
      );
      return;
    }

    setState(() => _isCreating = true);

    final group = await ref.read(groupsProvider.notifier).createGroup(
          name: name,
          description: _descriptionController.text.trim().isNotEmpty
              ? _descriptionController.text.trim()
              : null,
          memberIds: _selectedMemberIds.toList(),
        );

    setState(() => _isCreating = false);

    if (group != null && mounted) {
      Navigator.pop(context);
      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (context) => GroupChatScreen(group: group),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final contactsState = ref.watch(contactsProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Create Group'),
        actions: [
          TextButton(
            onPressed: _isCreating ? null : _createGroup,
            child: _isCreating
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('Create'),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            controller: _nameController,
            decoration: const InputDecoration(
              labelText: 'Group Name',
              hintText: 'Enter group name',
              border: OutlineInputBorder(),
            ),
            textCapitalization: TextCapitalization.words,
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _descriptionController,
            decoration: const InputDecoration(
              labelText: 'Description (optional)',
              hintText: 'What is this group about?',
              border: OutlineInputBorder(),
            ),
            maxLines: 2,
          ),
          const SizedBox(height: 24),
          Text(
            'Add Members',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          Text(
            'Select contacts to add to the group',
            style: TextStyle(color: Colors.grey[600], fontSize: 12),
          ),
          const SizedBox(height: 16),
          if (contactsState.isLoading)
            const Center(child: CircularProgressIndicator())
          else if (contactsState.contacts.isEmpty)
            Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Column(
                  children: [
                    Icon(Icons.people_outline, size: 48, color: Colors.grey[400]),
                    const SizedBox(height: 16),
                    Text(
                      'No contacts yet',
                      style: TextStyle(color: Colors.grey[600]),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Add contacts first to create a group',
                      style: TextStyle(color: Colors.grey[500], fontSize: 12),
                    ),
                  ],
                ),
              ),
            )
          else
            ...contactsState.contacts.map((contact) => _ContactTile(
                  contact: contact,
                  isSelected: _selectedMemberIds.contains(contact.contactId),
                  onToggle: () {
                    setState(() {
                      if (_selectedMemberIds.contains(contact.contactId)) {
                        _selectedMemberIds.remove(contact.contactId);
                      } else {
                        _selectedMemberIds.add(contact.contactId);
                      }
                    });
                  },
                )),
          if (_selectedMemberIds.isNotEmpty) ...[
            const SizedBox(height: 16),
            Text(
              '${_selectedMemberIds.length} member${_selectedMemberIds.length > 1 ? 's' : ''} selected',
              style: TextStyle(color: Colors.grey[600]),
            ),
          ],
        ],
      ),
    );
  }
}

class _ContactTile extends StatelessWidget {
  final Contact contact;
  final bool isSelected;
  final VoidCallback onToggle;

  const _ContactTile({
    required this.contact,
    required this.isSelected,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: CircleAvatar(
        child: Text(contact.displayName[0].toUpperCase()),
      ),
      title: Text(contact.displayName),
      trailing: Checkbox(
        value: isSelected,
        onChanged: (_) => onToggle(),
      ),
      onTap: onToggle,
    );
  }
}
