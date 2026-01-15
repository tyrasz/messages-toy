import 'package:flutter/material.dart';
import '../models/pinned_message.dart';

class PinnedMessageBanner extends StatelessWidget {
  final PinnedMessage pinned;
  final VoidCallback onTap;
  final VoidCallback? onUnpin;

  const PinnedMessageBanner({
    super.key,
    required this.pinned,
    required this.onTap,
    this.onUnpin,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.blue.shade50,
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            children: [
              Container(
                width: 3,
                height: 32,
                decoration: BoxDecoration(
                  color: Colors.blue,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(width: 10),
              const Icon(Icons.push_pin, size: 16, color: Colors.blue),
              const SizedBox(width: 8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      pinned.senderName ?? 'Message',
                      style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: Colors.blue,
                      ),
                    ),
                    Text(
                      pinned.messageContent ?? 'Pinned message',
                      style: TextStyle(
                        fontSize: 13,
                        color: Colors.grey.shade700,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              if (onUnpin != null)
                IconButton(
                  icon: const Icon(Icons.close, size: 18),
                  onPressed: onUnpin,
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                  color: Colors.grey.shade600,
                ),
            ],
          ),
        ),
      ),
    );
  }
}
