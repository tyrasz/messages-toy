import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/user.dart';
import '../providers/auth_provider.dart';

class ProfileScreen extends ConsumerStatefulWidget {
  final User? user; // If null, show current user's profile

  const ProfileScreen({super.key, this.user});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  late TextEditingController _displayNameController;
  late TextEditingController _aboutController;
  String _selectedEmoji = '';
  bool _isEditing = false;
  bool _isSaving = false;

  bool get isOwnProfile => widget.user == null;

  User get displayUser => widget.user ?? ref.watch(authProvider).user!;

  @override
  void initState() {
    super.initState();
    _displayNameController = TextEditingController(text: displayUser.displayName ?? '');
    _aboutController = TextEditingController(text: displayUser.about ?? '');
    _selectedEmoji = displayUser.statusEmoji ?? '';
  }

  @override
  void dispose() {
    _displayNameController.dispose();
    _aboutController.dispose();
    super.dispose();
  }

  Future<void> _saveProfile() async {
    setState(() => _isSaving = true);

    try {
      final apiService = ref.read(apiServiceProvider);
      await apiService.updateProfile(
        displayName: _displayNameController.text.trim().isEmpty
            ? null
            : _displayNameController.text.trim(),
        about: _aboutController.text.trim(),
        statusEmoji: _selectedEmoji,
      );

      setState(() {
        _isEditing = false;
        _isSaving = false;
      });

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Profile updated')),
        );
      }
    } catch (e) {
      setState(() => _isSaving = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Failed to update profile')),
        );
      }
    }
  }

  void _showEmojiPicker() {
    final emojis = ['', '😊', '😎', '🏃', '💼', '🎮', '📚', '🎵', '✈️', '🌙', '🔴', '🟢'];

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Select status emoji',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 16),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: emojis.map((emoji) {
                  final isSelected = _selectedEmoji == emoji;
                  return GestureDetector(
                    onTap: () {
                      setState(() => _selectedEmoji = emoji);
                      Navigator.pop(context);
                    },
                    child: Container(
                      width: 48,
                      height: 48,
                      decoration: BoxDecoration(
                        color: isSelected ? Colors.blue.shade100 : Colors.grey.shade100,
                        borderRadius: BorderRadius.circular(12),
                        border: isSelected
                            ? Border.all(color: Colors.blue, width: 2)
                            : null,
                      ),
                      child: Center(
                        child: emoji.isEmpty
                            ? Icon(Icons.close, color: Colors.grey.shade400)
                            : Text(emoji, style: const TextStyle(fontSize: 24)),
                      ),
                    ),
                  );
                }).toList(),
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(isOwnProfile ? 'My Profile' : 'Profile'),
        actions: isOwnProfile
            ? [
                if (_isEditing)
                  TextButton(
                    onPressed: _isSaving ? null : _saveProfile,
                    child: _isSaving
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Save'),
                  )
                else
                  IconButton(
                    icon: const Icon(Icons.edit),
                    onPressed: () => setState(() => _isEditing = true),
                  ),
              ]
            : null,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            // Avatar
            Stack(
              children: [
                CircleAvatar(
                  radius: 60,
                  backgroundColor: Theme.of(context).primaryColor,
                  child: Text(
                    displayUser.displayNameOrUsername[0].toUpperCase(),
                    style: const TextStyle(
                      fontSize: 48,
                      color: Colors.white,
                    ),
                  ),
                ),
                if (_selectedEmoji.isNotEmpty)
                  Positioned(
                    bottom: 0,
                    right: 0,
                    child: Container(
                      padding: const EdgeInsets.all(4),
                      decoration: BoxDecoration(
                        color: Colors.white,
                        shape: BoxShape.circle,
                        boxShadow: [
                          BoxShadow(
                            color: Colors.black.withOpacity(0.2),
                            blurRadius: 4,
                          ),
                        ],
                      ),
                      child: Text(_selectedEmoji, style: const TextStyle(fontSize: 24)),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 24),

            // Username (non-editable)
            ListTile(
              leading: const Icon(Icons.alternate_email),
              title: const Text('Username'),
              subtitle: Text('@${displayUser.username}'),
            ),
            const Divider(),

            // Display name
            if (_isEditing)
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: TextField(
                  controller: _displayNameController,
                  decoration: const InputDecoration(
                    labelText: 'Display Name',
                    hintText: 'Enter your display name',
                    prefixIcon: Icon(Icons.person),
                    border: OutlineInputBorder(),
                  ),
                ),
              )
            else
              ListTile(
                leading: const Icon(Icons.person),
                title: const Text('Display Name'),
                subtitle: Text(displayUser.displayName ?? 'Not set'),
              ),

            const Divider(),

            // About/Status
            if (_isEditing)
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: TextField(
                  controller: _aboutController,
                  maxLines: 3,
                  maxLength: 500,
                  decoration: const InputDecoration(
                    labelText: 'About',
                    hintText: 'Tell something about yourself...',
                    prefixIcon: Icon(Icons.info_outline),
                    border: OutlineInputBorder(),
                  ),
                ),
              )
            else
              ListTile(
                leading: const Icon(Icons.info_outline),
                title: const Text('About'),
                subtitle: Text(displayUser.about ?? 'Hey there! I\'m using this app.'),
              ),

            const Divider(),

            // Status emoji
            if (_isEditing)
              ListTile(
                leading: const Icon(Icons.emoji_emotions),
                title: const Text('Status Emoji'),
                subtitle: Text(_selectedEmoji.isEmpty ? 'None' : _selectedEmoji),
                trailing: const Icon(Icons.chevron_right),
                onTap: _showEmojiPicker,
              )
            else
              ListTile(
                leading: const Icon(Icons.emoji_emotions),
                title: const Text('Status'),
                subtitle: Row(
                  children: [
                    if (displayUser.statusEmoji?.isNotEmpty ?? false) ...[
                      Text(displayUser.statusEmoji!),
                      const SizedBox(width: 8),
                    ],
                    Text(
                      displayUser.online ? 'Online' : 'Offline',
                      style: TextStyle(
                        color: displayUser.online ? Colors.green : Colors.grey,
                      ),
                    ),
                  ],
                ),
              ),

            if (!isOwnProfile) ...[
              const Divider(),
              const SizedBox(height: 16),

              // Action buttons for other user's profile
              Row(
                children: [
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: () {
                        // Navigate to chat
                        Navigator.pop(context);
                      },
                      icon: const Icon(Icons.message),
                      label: const Text('Message'),
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}
