import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:timeago/timeago.dart' as timeago;
import '../models/starred_message.dart';
import '../providers/starred_provider.dart';
import '../providers/auth_provider.dart';
import 'chat_screen.dart';

class StarredMessagesScreen extends ConsumerStatefulWidget {
  const StarredMessagesScreen({super.key});

  @override
  ConsumerState<StarredMessagesScreen> createState() => _StarredMessagesScreenState();
}

class _StarredMessagesScreenState extends ConsumerState<StarredMessagesScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(starredProvider.notifier).loadStarredMessages();
    });
  }

  @override
  Widget build(BuildContext context) {
    final starredState = ref.watch(starredProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Starred Messages'),
      ),
      body: starredState.isLoading
          ? const Center(child: CircularProgressIndicator())
          : starredState.messages.isEmpty
              ? _buildEmptyState()
              : RefreshIndicator(
                  onRefresh: () =>
                      ref.read(starredProvider.notifier).loadStarredMessages(),
                  child: ListView.builder(
                    itemCount: starredState.messages.length,
                    itemBuilder: (context, index) {
                      final starred = starredState.messages[index];
                      return _StarredMessageTile(
                        starred: starred,
                        onUnstar: () {
                          ref.read(starredProvider.notifier).unstarMessage(
                                starred.message.id,
                              );
                        },
                      );
                    },
                  ),
                ),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.star_outline,
            size: 64,
            color: Colors.grey[400],
          ),
          const SizedBox(height: 16),
          Text(
            'No starred messages',
            style: TextStyle(
              fontSize: 18,
              color: Colors.grey[600],
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Long press any message to star it',
            style: TextStyle(
              color: Colors.grey[500],
            ),
          ),
        ],
      ),
    );
  }
}

class _StarredMessageTile extends ConsumerWidget {
  final StarredMessage starred;
  final VoidCallback onUnstar;

  const _StarredMessageTile({
    required this.starred,
    required this.onUnstar,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authProvider);
    final isMe = starred.message.senderId == authState.user?.id;

    return Dismissible(
      key: Key(starred.id),
      direction: DismissDirection.endToStart,
      background: Container(
        alignment: Alignment.centerRight,
        padding: const EdgeInsets.only(right: 20),
        color: Colors.red,
        child: const Icon(
          Icons.star_outline,
          color: Colors.white,
        ),
      ),
      onDismissed: (_) => onUnstar(),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: starred.isGroupMessage
              ? Colors.blue[100]
              : Colors.grey[200],
          child: Icon(
            starred.isGroupMessage ? Icons.group : Icons.person,
            color: starred.isGroupMessage ? Colors.blue[600] : Colors.grey[600],
          ),
        ),
        title: Row(
          children: [
            Expanded(
              child: Text(
                starred.conversationName,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontWeight: FontWeight.w500),
              ),
            ),
            Text(
              timeago.format(starred.message.createdAt),
              style: TextStyle(
                fontSize: 12,
                color: Colors.grey[500],
              ),
            ),
          ],
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!isMe)
              Text(
                'From: ${starred.message.senderId.substring(0, 8)}',
                style: TextStyle(
                  fontSize: 11,
                  color: Colors.grey[500],
                ),
              ),
            Text(
              starred.message.content ?? '[Media]',
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: Colors.grey[600]),
            ),
          ],
        ),
        trailing: IconButton(
          icon: const Icon(Icons.star, color: Colors.amber),
          onPressed: onUnstar,
        ),
        onTap: () {
          if (starred.user != null) {
            Navigator.push(
              context,
              MaterialPageRoute(
                builder: (context) => ChatScreen(user: starred.user!),
              ),
            );
          }
          // TODO: Navigate to group chat if group message
        },
      ),
    );
  }
}
