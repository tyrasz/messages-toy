import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:file_picker/file_picker.dart';
import '../models/user.dart';
import '../models/message.dart';
import '../models/reaction.dart';
import '../providers/auth_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/blocks_provider.dart';
import '../widgets/media_bubbles.dart';
import 'forward_message_screen.dart';

class ChatScreen extends ConsumerStatefulWidget {
  final User user;

  const ChatScreen({super.key, required this.user});

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  Timer? _typingTimer;
  bool _isTyping = false;
  Message? _replyingTo;

  @override
  void initState() {
    super.initState();
    // Load messages for this conversation
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(chatProvider.notifier).loadMessages(widget.user.id);
    });

    _messageController.addListener(_onTextChanged);
  }

  @override
  void dispose() {
    _messageController.removeListener(_onTextChanged);
    _messageController.dispose();
    _scrollController.dispose();
    _typingTimer?.cancel();
    super.dispose();
  }

  void _onTextChanged() {
    if (_messageController.text.isNotEmpty && !_isTyping) {
      _isTyping = true;
      ref.read(chatProvider.notifier).sendTyping(widget.user.id, true);
    }

    _typingTimer?.cancel();
    _typingTimer = Timer(const Duration(seconds: 2), () {
      if (_isTyping) {
        _isTyping = false;
        ref.read(chatProvider.notifier).sendTyping(widget.user.id, false);
      }
    });
  }

  void _sendMessage() {
    final content = _messageController.text.trim();
    if (content.isEmpty) return;

    ref.read(chatProvider.notifier).sendMessage(
          widget.user.id,
          content: content,
          replyToId: _replyingTo?.id,
        );

    _messageController.clear();
    _isTyping = false;
    setState(() => _replyingTo = null);
  }

  void _onReply(Message message) {
    setState(() => _replyingTo = message);
  }

  void _cancelReply() {
    setState(() => _replyingTo = null);
  }

  void _onEdit(Message message) {
    final controller = TextEditingController(text: message.content);
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Edit Message'),
        content: TextField(
          controller: controller,
          maxLines: 4,
          autofocus: true,
          decoration: const InputDecoration(
            hintText: 'Edit your message...',
            border: OutlineInputBorder(),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              final newContent = controller.text.trim();
              if (newContent.isNotEmpty && newContent != message.content) {
                ref.read(chatProvider.notifier).editMessage(message.id, newContent);
              }
              Navigator.pop(context);
            },
            child: const Text('Save'),
          ),
        ],
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
                  color: Colors.purple[100],
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.photo, color: Colors.purple[600]),
              ),
              title: const Text('Photo'),
              subtitle: const Text('Send an image from gallery'),
              onTap: () {
                Navigator.pop(context);
                _pickAndSendImage();
              },
            ),
            ListTile(
              leading: Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: Colors.red[100],
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.videocam, color: Colors.red[600]),
              ),
              title: const Text('Video'),
              subtitle: const Text('Send a video'),
              onTap: () {
                Navigator.pop(context);
                _pickAndSendVideo();
              },
            ),
            ListTile(
              leading: Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: Colors.blue[100],
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.insert_drive_file, color: Colors.blue[600]),
              ),
              title: const Text('Document'),
              subtitle: const Text('Send PDF, DOC, or other files'),
              onTap: () {
                Navigator.pop(context);
                _pickAndSendDocument();
              },
            ),
            ListTile(
              leading: Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: Colors.orange[100],
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.mic, color: Colors.orange[600]),
              ),
              title: const Text('Audio'),
              subtitle: const Text('Send an audio file'),
              onTap: () {
                Navigator.pop(context);
                _pickAndSendAudio();
              },
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _pickAndSendImage() async {
    final picker = ImagePicker();
    final image = await picker.pickImage(source: ImageSource.gallery);

    if (image != null) {
      await _uploadAndSendMedia(image.path, 'image');
    }
  }

  Future<void> _pickAndSendVideo() async {
    final picker = ImagePicker();
    final video = await picker.pickVideo(source: ImageSource.gallery);

    if (video != null) {
      await _uploadAndSendMedia(video.path, 'video');
    }
  }

  Future<void> _pickAndSendDocument() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'txt'],
    );

    if (result != null && result.files.single.path != null) {
      await _uploadAndSendMedia(result.files.single.path!, 'document');
    }
  }

  Future<void> _pickAndSendAudio() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.audio,
    );

    if (result != null && result.files.single.path != null) {
      await _uploadAndSendMedia(result.files.single.path!, 'audio');
    }
  }

  Future<void> _uploadAndSendMedia(String filePath, String mediaType) async {
    final apiService = ref.read(apiServiceProvider);
    try {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Uploading $mediaType...')),
      );

      final result = await apiService.uploadMedia(filePath);
      final mediaId = result['id'] as String;
      final status = result['status'] as String?;

      if (status == 'pending') {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Media uploaded, pending moderation...')),
        );
      }

      // Send message with media ID
      ref.read(chatProvider.notifier).sendMessage(
            widget.user.id,
            mediaId: mediaId,
            replyToId: _replyingTo?.id,
          );
      setState(() => _replyingTo = null);
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Failed to upload: $e')),
      );
    }
  }

  Future<void> _handleBlockUser() async {
    final blocksNotifier = ref.read(blocksProvider.notifier);
    final isBlocked = ref.read(blocksProvider).isUserBlocked(widget.user.id);

    if (isBlocked) {
      final success = await blocksNotifier.unblockUser(widget.user.id);
      if (success && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${widget.user.displayNameOrUsername} unblocked')),
        );
      }
    } else {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('Block User'),
          content: Text(
            'Are you sure you want to block ${widget.user.displayNameOrUsername}? '
            'They won\'t be able to send you messages or see when you\'re online.',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Cancel'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context, true),
              style: TextButton.styleFrom(foregroundColor: Colors.red),
              child: const Text('Block'),
            ),
          ],
        ),
      );

      if (confirmed == true) {
        final success = await blocksNotifier.blockUser(widget.user.id);
        if (success && mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('${widget.user.displayNameOrUsername} blocked')),
          );
        }
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final chatState = ref.watch(chatProvider);
    final authState = ref.watch(authProvider);
    final blocksState = ref.watch(blocksProvider);
    final messages = chatState.messages[widget.user.id] ?? [];
    final isTyping = chatState.typingStatus[widget.user.id] ?? false;
    final isBlocked = blocksState.isUserBlocked(widget.user.id);

    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            CircleAvatar(
              radius: 18,
              child: Text(widget.user.displayNameOrUsername[0].toUpperCase()),
            ),
            const SizedBox(width: 12),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.user.displayNameOrUsername,
                  style: const TextStyle(fontSize: 16),
                ),
                Text(
                  isBlocked
                      ? 'Blocked'
                      : widget.user.online
                          ? 'Online'
                          : isTyping
                              ? 'Typing...'
                              : 'Offline',
                  style: TextStyle(
                    fontSize: 12,
                    color: isBlocked
                        ? Colors.red
                        : widget.user.online
                            ? Colors.green
                            : Colors.grey,
                  ),
                ),
              ],
            ),
          ],
        ),
        actions: [
          PopupMenuButton<String>(
            onSelected: (value) {
              if (value == 'block') {
                _handleBlockUser();
              }
            },
            itemBuilder: (context) => [
              PopupMenuItem(
                value: 'block',
                child: Row(
                  children: [
                    Icon(
                      isBlocked ? Icons.lock_open : Icons.block,
                      color: isBlocked ? Colors.green : Colors.red,
                    ),
                    const SizedBox(width: 8),
                    Text(isBlocked ? 'Unblock' : 'Block'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: Column(
        children: [
          // Messages list
          Expanded(
            child: messages.isEmpty
                ? Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.chat_bubble_outline,
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
                          'Say hello!',
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
                      return _SwipeableMessageBubble(
                        message: message,
                        isMe: isMe,
                        currentUserId: authState.user?.id,
                        otherUserName: widget.user.displayNameOrUsername,
                        onReply: () => _onReply(message),
                        onEdit: _onEdit,
                      );
                    },
                  ),
          ),

          // Typing indicator
          if (isTyping)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              alignment: Alignment.centerLeft,
              child: Row(
                children: [
                  Text(
                    '${widget.user.displayNameOrUsername} is typing',
                    style: TextStyle(
                      color: Colors.grey[600],
                      fontSize: 12,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                  const SizedBox(width: 8),
                  const _TypingIndicator(),
                ],
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
                          _replyingTo!.senderId == authState.user?.id
                              ? 'Replying to yourself'
                              : 'Replying to ${widget.user.displayNameOrUsername}',
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

          // Message input or blocked message
          if (isBlocked)
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: Colors.red[50],
                border: Border(
                  top: BorderSide(color: Colors.red[200]!),
                ),
              ),
              child: SafeArea(
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(Icons.block, color: Colors.red[400], size: 20),
                    const SizedBox(width: 8),
                    Text(
                      'You have blocked this user',
                      style: TextStyle(color: Colors.red[700]),
                    ),
                    const SizedBox(width: 8),
                    TextButton(
                      onPressed: _handleBlockUser,
                      child: const Text('Unblock'),
                    ),
                  ],
                ),
              ),
            )
          else
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
                          hintText: 'Type a message...',
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

class _SwipeableMessageBubble extends ConsumerWidget {
  final Message message;
  final bool isMe;
  final String? currentUserId;
  final String otherUserName;
  final VoidCallback onReply;
  final void Function(Message) onEdit;

  const _SwipeableMessageBubble({
    required this.message,
    required this.isMe,
    required this.currentUserId,
    required this.otherUserName,
    required this.onReply,
    required this.onEdit,
  });

  void _showMessageOptions(BuildContext context, WidgetRef ref) {
    if (message.isDeleted) return;

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Quick reaction row
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: QuickReactions.defaults.map((emoji) {
                  return GestureDetector(
                    onTap: () {
                      Navigator.pop(context);
                      ref.read(chatProvider.notifier).addReaction(message.id, emoji);
                    },
                    child: Container(
                      padding: const EdgeInsets.all(8),
                      decoration: BoxDecoration(
                        color: Colors.grey[100],
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(emoji, style: const TextStyle(fontSize: 24)),
                    ),
                  );
                }).toList(),
              ),
            ),
            const Divider(height: 1),
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
                      originalSenderName: isMe ? 'You' : otherUserName,
                    ),
                  ),
                );
              },
            ),
            if (isMe && message.content != null) ...[
              ListTile(
                leading: const Icon(Icons.edit),
                title: const Text('Edit'),
                onTap: () {
                  Navigator.pop(context);
                  onEdit(message);
                },
              ),
            ],
            ListTile(
              leading: const Icon(Icons.delete_outline),
              title: const Text('Delete for me'),
              onTap: () {
                Navigator.pop(context);
                ref.read(chatProvider.notifier).deleteMessage(message.id, forEveryone: false);
              },
            ),
            if (isMe)
              ListTile(
                leading: Icon(Icons.delete_forever, color: Colors.red[400]),
                title: Text('Delete for everyone', style: TextStyle(color: Colors.red[400])),
                onTap: () {
                  Navigator.pop(context);
                  ref.read(chatProvider.notifier).deleteMessage(message.id, forEveryone: true);
                },
              ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return GestureDetector(
      onLongPress: () => _showMessageOptions(context, ref),
      child: Dismissible(
        key: Key(message.id),
        direction: message.isDeleted
            ? DismissDirection.none
            : (isMe ? DismissDirection.endToStart : DismissDirection.startToEnd),
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
        child: _MessageBubble(
          message: message,
          isMe: isMe,
          currentUserId: currentUserId,
        ),
      ),
    );
  }
}

class _MessageBubble extends ConsumerWidget {
  final Message message;
  final bool isMe;
  final String? currentUserId;

  const _MessageBubble({
    required this.message,
    required this.isMe,
    this.currentUserId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Align(
      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
      child: Column(
        crossAxisAlignment: isMe ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          Container(
            margin: EdgeInsets.only(
              top: 4,
              bottom: message.hasReactions ? 0 : 4,
            ),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            constraints: BoxConstraints(
              maxWidth: MediaQuery.of(context).size.width * 0.75,
            ),
            decoration: BoxDecoration(
              color: isMe
                  ? Theme.of(context).primaryColor
                  : Colors.grey[200],
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
                      message.replyTo!.senderId == currentUserId ? 'You' : 'Them',
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
            // Message content
            if (message.isDeleted)
              Text(
                '[Message deleted]',
                style: TextStyle(
                  color: isMe ? Colors.white60 : Colors.grey[500],
                  fontStyle: FontStyle.italic,
                ),
              )
            else ...[
              // Media content
              if (message.hasMedia) ...[
                _buildMediaContent(context),
                if (message.content != null && message.content!.isNotEmpty)
                  const SizedBox(height: 8),
              ],
              // Text content
              if (message.content != null && message.content!.isNotEmpty)
                Text(
                  message.content!,
                  style: TextStyle(
                    color: isMe ? Colors.white : Colors.black87,
                  ),
                ),
            ],
            const SizedBox(height: 4),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  _formatTime(message.createdAt),
                  style: TextStyle(
                    fontSize: 10,
                    color: isMe ? Colors.white70 : Colors.grey[600],
                  ),
                ),
                if (message.isEdited && !message.isDeleted) ...[
                  const SizedBox(width: 4),
                  Text(
                    '(edited)',
                    style: TextStyle(
                      fontSize: 10,
                      color: isMe ? Colors.white60 : Colors.grey[500],
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],
                if (isMe && !message.isDeleted) ...[
                  const SizedBox(width: 4),
                  Icon(
                    _getStatusIcon(message.status),
                    size: 14,
                    color: message.status == MessageStatus.read
                        ? Colors.blue[200]
                        : Colors.white70,
                  ),
                ],
              ],
            ),
          ],
            ),
          ),
          // Reactions display
          if (message.hasReactions)
            _buildReactions(context, ref),
        ],
      ),
    );
  }

  Widget _buildReactions(BuildContext context, WidgetRef ref) {
    return Transform.translate(
      offset: const Offset(0, -8),
      child: Container(
        margin: EdgeInsets.only(
          left: isMe ? 0 : 8,
          right: isMe ? 8 : 0,
          bottom: 4,
        ),
        child: Wrap(
          spacing: 4,
          children: message.reactions.map((reaction) {
            final hasMyReaction = reaction.hasUserReacted(currentUserId ?? '');
            return GestureDetector(
              onTap: () {
                if (hasMyReaction) {
                  ref.read(chatProvider.notifier).removeReaction(message.id);
                } else {
                  ref.read(chatProvider.notifier).addReaction(message.id, reaction.emoji);
                }
              },
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: hasMyReaction
                      ? Theme.of(context).primaryColor.withOpacity(0.2)
                      : Colors.grey[200],
                  borderRadius: BorderRadius.circular(12),
                  border: hasMyReaction
                      ? Border.all(color: Theme.of(context).primaryColor, width: 1)
                      : null,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(reaction.emoji, style: const TextStyle(fontSize: 14)),
                    if (reaction.count > 1) ...[
                      const SizedBox(width: 2),
                      Text(
                        '${reaction.count}',
                        style: TextStyle(
                          fontSize: 11,
                          color: hasMyReaction
                              ? Theme.of(context).primaryColor
                              : Colors.grey[600],
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            );
          }).toList(),
        ),
      ),
    );
  }

  String _formatTime(DateTime dateTime) {
    final hour = dateTime.hour.toString().padLeft(2, '0');
    final minute = dateTime.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }

  IconData _getStatusIcon(MessageStatus status) {
    switch (status) {
      case MessageStatus.sent:
        return Icons.check;
      case MessageStatus.delivered:
        return Icons.done_all;
      case MessageStatus.read:
        return Icons.done_all;
    }
  }

  Widget _buildMediaContent(BuildContext context) {
    switch (message.mediaType) {
      case MediaType.image:
        return ImageMessageBubble(message: message, isMe: isMe);
      case MediaType.video:
        return VideoMessageBubble(message: message, isMe: isMe);
      case MediaType.audio:
        return AudioMessageBubble(message: message, isMe: isMe);
      case MediaType.document:
        return DocumentMessageBubble(message: message, isMe: isMe);
      case MediaType.none:
        // Fallback for media without type info
        if (message.mediaUrl != null) {
          return ImageMessageBubble(message: message, isMe: isMe);
        }
        return const SizedBox.shrink();
    }
  }
}

class _TypingIndicator extends StatefulWidget {
  const _TypingIndicator();

  @override
  State<_TypingIndicator> createState() => _TypingIndicatorState();
}

class _TypingIndicatorState extends State<_TypingIndicator>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(milliseconds: 1000),
      vsync: this,
    )..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: List.generate(3, (index) {
            final delay = index * 0.2;
            final animation = (_controller.value + delay) % 1.0;
            return Container(
              margin: const EdgeInsets.symmetric(horizontal: 2),
              width: 6,
              height: 6,
              decoration: BoxDecoration(
                color: Colors.grey.withOpacity(0.3 + animation * 0.7),
                shape: BoxShape.circle,
              ),
            );
          }),
        );
      },
    );
  }
}
