import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/group.dart';
import '../models/message.dart';
import '../providers/auth_provider.dart';
import '../providers/groups_provider.dart';

class GroupChatScreen extends ConsumerStatefulWidget {
  final Group group;

  const GroupChatScreen({super.key, required this.group});

  @override
  ConsumerState<GroupChatScreen> createState() => _GroupChatScreenState();
}

class _GroupChatScreenState extends ConsumerState<GroupChatScreen> {
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  Timer? _typingTimer;
  Message? _replyingTo;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(groupsProvider.notifier).loadGroupMessages(widget.group.id);
    });
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    _typingTimer?.cancel();
    super.dispose();
  }

  void _sendMessage() {
    final content = _messageController.text.trim();
    if (content.isEmpty) return;

    final webSocketService = ref.read(webSocketServiceProvider);
    webSocketService.sendMessage(
      groupId: widget.group.id,
      content: content,
      replyToId: _replyingTo?.id,
    );

    _messageController.clear();
    setState(() => _replyingTo = null);
  }

  void _onReply(Message message) {
    setState(() => _replyingTo = message);
  }

  void _cancelReply() {
    setState(() => _replyingTo = null);
  }

  void _showGroupInfo() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => DraggableScrollableSheet(
        initialChildSize: 0.6,
        minChildSize: 0.3,
        maxChildSize: 0.9,
        expand: false,
        builder: (context, scrollController) => _GroupInfoSheet(
          group: widget.group,
          scrollController: scrollController,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final groupsState = ref.watch(groupsProvider);
    final authState = ref.watch(authProvider);
    final messages = groupsState.messages[widget.group.id] ?? [];

    return Scaffold(
      appBar: AppBar(
        title: InkWell(
          onTap: _showGroupInfo,
          child: Row(
            children: [
              CircleAvatar(
                radius: 18,
                backgroundColor: Theme.of(context).primaryColor,
                child: Text(
                  widget.group.name[0].toUpperCase(),
                  style: const TextStyle(color: Colors.white),
                ),
              ),
              const SizedBox(width: 12),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    widget.group.name,
                    style: const TextStyle(fontSize: 16),
                  ),
                  Text(
                    '${widget.group.memberCount} members',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey[600],
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.info_outline),
            onPressed: _showGroupInfo,
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: messages.isEmpty
                ? Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.group,
                          size: 48,
                          color: Colors.grey[400],
                        ),
                        const SizedBox(height: 16),
                        Text(
                          'No messages yet',
                          style: TextStyle(color: Colors.grey[600]),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'Start the conversation!',
                          style: TextStyle(color: Colors.grey[500]),
                        ),
                      ],
                    ),
                  )
                : ListView.builder(
                    controller: _scrollController,
                    reverse: true,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    itemCount: messages.length,
                    itemBuilder: (context, index) {
                      final message = messages[index];
                      final isMe = message.senderId == authState.user?.id;
                      return _SwipeableGroupMessageBubble(
                        message: message,
                        isMe: isMe,
                        showSender: !isMe,
                        currentUserId: authState.user?.id,
                        onReply: () => _onReply(message),
                      );
                    },
                  ),
          ),

          // Reply preview
          if (_replyingTo != null)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.grey[100],
                border: Border(
                  left: BorderSide(
                    color: Theme.of(context).primaryColor,
                    width: 4,
                  ),
                ),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          _replyingTo!.senderId == ref.watch(authProvider).user?.id
                              ? 'Replying to yourself'
                              : 'Replying to ${_replyingTo!.senderId.substring(0, 8)}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Theme.of(context).primaryColor,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          _replyingTo!.content ?? '[Media]',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 13,
                            color: Colors.grey[600],
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 20),
                    onPressed: _cancelReply,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(),
                  ),
                ],
              ),
            ),

          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: Theme.of(context).scaffoldBackgroundColor,
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withOpacity(0.05),
                  blurRadius: 4,
                  offset: const Offset(0, -2),
                ),
              ],
            ),
            child: SafeArea(
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _messageController,
                      decoration: InputDecoration(
                        hintText: 'Message ${widget.group.name}...',
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(24),
                          borderSide: BorderSide.none,
                        ),
                        filled: true,
                        fillColor: Colors.grey[200],
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 16,
                          vertical: 8,
                        ),
                      ),
                      maxLines: 4,
                      minLines: 1,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => _sendMessage(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    icon: const Icon(Icons.send),
                    onPressed: _sendMessage,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SwipeableGroupMessageBubble extends StatelessWidget {
  final Message message;
  final bool isMe;
  final bool showSender;
  final String? currentUserId;
  final VoidCallback onReply;

  const _SwipeableGroupMessageBubble({
    required this.message,
    required this.isMe,
    required this.showSender,
    required this.currentUserId,
    required this.onReply,
  });

  @override
  Widget build(BuildContext context) {
    return Dismissible(
      key: Key(message.id),
      direction: isMe ? DismissDirection.endToStart : DismissDirection.startToEnd,
      confirmDismiss: (direction) async {
        onReply();
        return false; // Don't actually dismiss
      },
      background: Container(
        alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
        padding: EdgeInsets.only(
          left: isMe ? 0 : 20,
          right: isMe ? 20 : 0,
        ),
        child: Icon(
          Icons.reply,
          color: Colors.grey[400],
        ),
      ),
      child: _GroupMessageBubble(
        message: message,
        isMe: isMe,
        showSender: showSender,
        currentUserId: currentUserId,
      ),
    );
  }
}

class _GroupMessageBubble extends StatelessWidget {
  final Message message;
  final bool isMe;
  final bool showSender;
  final String? currentUserId;

  const _GroupMessageBubble({
    required this.message,
    required this.isMe,
    this.showSender = false,
    this.currentUserId,
  });

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        constraints: BoxConstraints(
          maxWidth: MediaQuery.of(context).size.width * 0.75,
        ),
        decoration: BoxDecoration(
          color: isMe ? Theme.of(context).primaryColor : Colors.grey[200],
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: Radius.circular(isMe ? 16 : 4),
            bottomRight: Radius.circular(isMe ? 4 : 16),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (showSender)
              Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text(
                  message.senderId.substring(0, 8), // TODO: Show actual username
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.bold,
                    color: isMe ? Colors.white70 : Colors.grey[700],
                  ),
                ),
              ),
            // Reply preview
            if (message.replyTo != null) ...[
              Container(
                padding: const EdgeInsets.all(8),
                margin: const EdgeInsets.only(bottom: 8),
                decoration: BoxDecoration(
                  color: isMe
                      ? Colors.white.withOpacity(0.2)
                      : Colors.grey[300],
                  borderRadius: BorderRadius.circular(8),
                  border: Border(
                    left: BorderSide(
                      color: isMe ? Colors.white70 : Theme.of(context).primaryColor,
                      width: 3,
                    ),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      message.replyTo!.senderId == currentUserId
                          ? 'You'
                          : message.replyTo!.senderId.substring(0, 8),
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                        color: isMe ? Colors.white70 : Theme.of(context).primaryColor,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      message.replyTo!.content ?? '[Media]',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: isMe ? Colors.white60 : Colors.grey[600],
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (message.content != null)
              Text(
                message.content!,
                style: TextStyle(
                  color: isMe ? Colors.white : Colors.black87,
                ),
              ),
            const SizedBox(height: 4),
            Text(
              _formatTime(message.createdAt),
              style: TextStyle(
                fontSize: 10,
                color: isMe ? Colors.white70 : Colors.grey[600],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _formatTime(DateTime dateTime) {
    final hour = dateTime.hour.toString().padLeft(2, '0');
    final minute = dateTime.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }
}

class _GroupInfoSheet extends ConsumerWidget {
  final Group group;
  final ScrollController scrollController;

  const _GroupInfoSheet({
    required this.group,
    required this.scrollController,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      child: Column(
        children: [
          Container(
            margin: const EdgeInsets.symmetric(vertical: 8),
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: Colors.grey[300],
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          Expanded(
            child: ListView(
              controller: scrollController,
              padding: const EdgeInsets.all(16),
              children: [
                Center(
                  child: CircleAvatar(
                    radius: 40,
                    backgroundColor: Theme.of(context).primaryColor,
                    child: Text(
                      group.name[0].toUpperCase(),
                      style: const TextStyle(
                        fontSize: 32,
                        color: Colors.white,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Text(
                  group.name,
                  style: Theme.of(context).textTheme.headlineSmall,
                  textAlign: TextAlign.center,
                ),
                if (group.description != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    group.description!,
                    style: TextStyle(color: Colors.grey[600]),
                    textAlign: TextAlign.center,
                  ),
                ],
                const SizedBox(height: 24),
                Text(
                  '${group.memberCount} members',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                if (group.members != null)
                  ...group.members!.map((member) => ListTile(
                        leading: CircleAvatar(
                          child: Text(member.user.displayNameOrUsername[0].toUpperCase()),
                        ),
                        title: Text(member.user.displayNameOrUsername),
                        subtitle: Text(member.role.name),
                        trailing: member.isOwner
                            ? const Chip(label: Text('Owner'))
                            : member.isAdmin
                                ? const Chip(label: Text('Admin'))
                                : null,
                      )),
                const SizedBox(height: 24),
                if (group.myRole != GroupRole.owner)
                  OutlinedButton.icon(
                    onPressed: () {
                      Navigator.pop(context);
                      ref.read(groupsProvider.notifier).leaveGroup(group.id);
                      Navigator.pop(context); // Pop chat screen too
                    },
                    icon: const Icon(Icons.exit_to_app, color: Colors.red),
                    label: const Text(
                      'Leave Group',
                      style: TextStyle(color: Colors.red),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
