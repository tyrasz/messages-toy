import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/auth_provider.dart';

class ArchivedScreen extends ConsumerStatefulWidget {
  const ArchivedScreen({super.key});

  @override
  ConsumerState<ArchivedScreen> createState() => _ArchivedScreenState();
}

class _ArchivedScreenState extends ConsumerState<ArchivedScreen> {
  List<Map<String, dynamic>> _archived = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadArchived();
  }

  Future<void> _loadArchived() async {
    setState(() => _isLoading = true);
    try {
      final apiService = ref.read(apiServiceProvider);
      final archived = await apiService.getArchivedConversations();
      setState(() {
        _archived = archived;
        _isLoading = false;
      });
    } catch (e) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _unarchive(Map<String, dynamic> item) async {
    try {
      final apiService = ref.read(apiServiceProvider);

      if (item['type'] == 'group') {
        await apiService.unarchiveConversation(
          groupId: item['group']['id'] as String,
        );
      } else {
        await apiService.unarchiveConversation(
          otherUserId: item['user']['id'] as String,
        );
      }

      setState(() {
        _archived.removeWhere((a) => a['id'] == item['id']);
      });

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Conversation unarchived')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Failed to unarchive')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Archived Chats'),
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _archived.isEmpty
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.archive_outlined,
                        size: 64,
                        color: Colors.grey.shade400,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        'No archived chats',
                        style: TextStyle(
                          fontSize: 16,
                          color: Colors.grey.shade600,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Archived conversations will appear here',
                        style: TextStyle(
                          fontSize: 14,
                          color: Colors.grey.shade500,
                        ),
                      ),
                    ],
                  ),
                )
              : ListView.builder(
                  itemCount: _archived.length,
                  itemBuilder: (context, index) {
                    final item = _archived[index];
                    final isGroup = item['type'] == 'group';

                    String name;
                    String subtitle;
                    IconData icon;

                    if (isGroup) {
                      name = item['group']['name'] as String? ?? 'Unknown Group';
                      subtitle = 'Group';
                      icon = Icons.group;
                    } else {
                      final displayName = item['user']['display_name'] as String?;
                      final username = item['user']['username'] as String? ?? 'Unknown';
                      name = displayName ?? username;
                      subtitle = '@$username';
                      icon = Icons.person;
                    }

                    return ListTile(
                      leading: CircleAvatar(
                        backgroundColor: Theme.of(context).primaryColor,
                        child: Icon(icon, color: Colors.white),
                      ),
                      title: Text(name),
                      subtitle: Text(subtitle),
                      trailing: IconButton(
                        icon: const Icon(Icons.unarchive),
                        onPressed: () => _unarchive(item),
                        tooltip: 'Unarchive',
                      ),
                    );
                  },
                ),
    );
  }
}
