import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/group.dart';
import '../models/message.dart';
import '../models/link_preview.dart';
import '../models/pinned_message.dart';
import '../providers/auth_provider.dart';
import '../providers/groups_provider.dart';
import '../providers/poll_provider.dart';
import '../providers/pinned_provider.dart';
import '../services/websocket_service.dart';
import '../widgets/link_preview_widget.dart';
import '../widgets/poll_create_dialog.dart';
import '../widgets/pinned_message_banner.dart';
import 'forward_message_screen.dart';

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
  Timer? _typingDebounceTimer;
  bool _isTyping = false;
  Message? _replyingTo;
  StreamSubscription? _typingSubscription;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(groupsProvider.notifier).loadGroupMessages(widget.group.id);
      _setupTypingListener();
      // Load pinned message
      ref.read(pinnedProvider.notifier).loadPinnedMessage(groupId: widget.group.id);
    });
    _messageController.addListener(_onTextChanged);
  }

  void _setupTypingListener() {
    final webSocketService = ref.read(webSocketServiceProvider);
    _typingSubscription = webSocketService.typingStream.listen((event) {
      final groupId = event['group_id'] as String?;
      final from = event['from'] as String?;
      final typing = event['typing'] as bool? ?? false;

      // Only handle typing events for this group
      if (groupId == widget.group.id && from != null) {
        ref.read(groupsProvider.notifier).handleGroupTyping(
          groupId: groupId,
          userId: from,
          isTyping: typing,
        );

        // Auto-clear typing indicator after 5 seconds
        if (typing) {
          Future.delayed(const Duration(seconds: 5), () {
            if (mounted) {
              ref.read(groupsProvider.notifier).handleGroupTyping(
                groupId: groupId,
                userId: from,
                isTyping: false,
              );
            }
          });
        }
      }
    });
  }

  void _onTextChanged() {
    if (_messageController.text.isNotEmpty && !_isTyping) {
      _isTyping = true;
      _sendTypingIndicator(true);
    }

    _typingDebounceTimer?.cancel();
    _typingDebounceTimer = Timer(const Duration(seconds: 2), () {
      if (_isTyping) {
        _isTyping = false;
        _sendTypingIndicator(false);
      }
    });
  }

  void _sendTypingIndicator(bool typing) {
    final webSocketService = ref.read(webSocketServiceProvider);
    webSocketService.sendGroupTyping(
      groupId: widget.group.id,
      typing: typing,
    );
  }

  @override
  void dispose() {
    // Stop typing when leaving
    if (_isTyping) {
      _sendTypingIndicator(false);
    }
    _messageController.removeListener(_onTextChanged);
    _messageController.dispose();
    _scrollController.dispose();
    _typingTimer?.cancel();
    _typingDebounceTimer?.cancel();
    _typingSubscription?.cancel();
    ref.read(groupsProvider.notifier).clearGroupTyping(widget.group.id);
    super.dispose();
  }

  void _sendMessage() {
    final content = _messageController.text.trim();
    if (content.isEmpty) return;

    // Stop typing indicator when sending
    if (_isTyping) {
      _isTyping = false;
      _sendTypingIndicator(false);
    }
    _typingDebounceTimer?.cancel();

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

  void _showAttachmentMenu() {
    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: Colors.green[100],
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.poll, color: Colors.green[600]),
              ),
              title: const Text('Poll'),
              subtitle: const Text('Create a poll'),
              onTap: () {
                Navigator.pop(context);
                _showCreatePollDialog();
              },
            ),
          ],
        ),
      ),
    );
  }

  void _showCreatePollDialog() {
    showDialog(
      context: context,
      builder: (context) => PollCreateDialog(
        groupId: widget.group.id,
        onSubmit: (question, options, multiSelect, anonymous) async {
          final poll = await ref.read(pollProvider.notifier).createPoll(
            question: question,
            options: options,
            multiSelect: multiSelect,
            anonymous: anonymous,
            groupId: widget.group.id,
          );
          if (poll != null && mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('Poll created')),
            );
          }
        },
      ),
    );
  }

  void _scrollToMessage(String messageId) {
    final groupsState = ref.read(groupsProvider);
    final messages = groupsState.messages[widget.group.id] ?? [];
    final index = messages.indexWhere((m) => m.id == messageId);
    if (index != -1) {
      _scrollController.animateTo(
        index * 80.0, // Approximate height
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    }
  }

  Future<void> _pinMessage(String messageId) async {
    await ref.read(pinnedProvider.notifier).pinMessage(
      messageId: messageId,
      groupId: widget.group.id,
    );

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Message pinned')),
      );
    }
  }

  Future<void> _unpinMessage() async {
    await ref.read(pinnedProvider.notifier).unpinMessage(groupId: widget.group.id);

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Message unpinned')),
      );
    }
  }

  String _buildTypingText(Set<String> typingUsers) {
    if (typingUsers.isEmpty) return '';

    final userNames = typingUsers.map((id) => id.substring(0, 8)).toList();
    if (userNames.length == 1) {
      return '${userNames[0]} is typing...';
    } else if (userNames.length == 2) {
      return '${userNames[0]} and ${userNames[1]} are typing...';
    } else {
      return '${userNames.length} people are typing...';
    }
  }

  @override
  Widget build(BuildContext context) {
    final groupsState = ref.watch(groupsProvider);
    final authState = ref.watch(authProvider);
    final pinnedState = ref.watch(pinnedProvider);
    final messages = groupsState.messages[widget.group.id] ?? [];
    final typingUsers = groupsState.getTypingUsers(widget.group.id);
    final typingText = _buildTypingText(typingUsers);
    final pinnedMessage = pinnedState.getForGroup(widget.group.id);

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
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.group.name,
                      style: const TextStyle(fontSize: 16),
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (typingText.isNotEmpty)
                      Text(
                        typingText,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.green[700],
                          fontStyle: FontStyle.italic,
                        ),
                        overflow: TextOverflow.ellipsis,
                      )
                    else
                      Text(
                        '${widget.group.memberCount} members',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey[600],
                        ),
                      ),
                  ],
                ),
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
          // Pinned message banner
          if (pinnedMessage != null)
            PinnedMessageBanner(
              pinned: pinnedMessage,
              onTap: () => _scrollToMessage(pinnedMessage.messageId),
              onUnpin: _unpinMessage,
            ),
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
                        groupName: widget.group.name,
                        onReply: () => _onReply(message),
                        onPin: _pinMessage,
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
                  IconButton(
                    icon: const Icon(Icons.attach_file),
                    onPressed: _showAttachmentMenu,
                  ),
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
  final String groupName;
  final VoidCallback onReply;
  final void Function(String) onPin;

  const _SwipeableGroupMessageBubble({
    required this.message,
    required this.isMe,
    required this.showSender,
    required this.currentUserId,
    required this.groupName,
    required this.onReply,
    required this.onPin,
  });

  void _showMessageOptions(BuildContext context) {
    if (message.isDeleted) return;

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.reply),
              title: const Text('Reply'),
              onTap: () {
                Navigator.pop(context);
                onReply();
              },
            ),
            ListTile(
              leading: const Icon(Icons.forward),
              title: const Text('Forward'),
              onTap: () {
                Navigator.pop(context);
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => ForwardMessageScreen(
                      message: message,
                      originalSenderName: isMe ? 'You' : 'Group: $groupName',
                    ),
                  ),
                );
              },
            ),
            ListTile(
              leading: const Icon(Icons.push_pin),
              title: const Text('Pin'),
              onTap: () {
                Navigator.pop(context);
                onPin(message.id);
              },
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPress: () => _showMessageOptions(context),
      child: Dismissible(
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
            // Forwarded indicator
            if (message.isForwarded) ...[
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.forward,
                    size: 12,
                    color: isMe ? Colors.white60 : Colors.grey[500],
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Forwarded from ${message.forwardedFrom}',
                    style: TextStyle(
                      fontSize: 11,
                      fontStyle: FontStyle.italic,
                      color: isMe ? Colors.white60 : Colors.grey[500],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
            ],
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
            // Link preview
            if (message.content != null && UrlExtractor.hasUrl(message.content!))
              LinkPreviewWidget(
                url: UrlExtractor.getFirstUrl(message.content!)!,
                isMe: isMe,
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
