import 'package:flutter/material.dart';
import '../models/poll.dart';

class PollWidget extends StatelessWidget {
  final Poll poll;
  final bool isCreator;
  final Function(String optionId) onVote;
  final VoidCallback? onClose;

  const PollWidget({
    super.key,
    required this.poll,
    required this.isCreator,
    required this.onVote,
    this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(maxWidth: 300),
      decoration: BoxDecoration(
        color: Colors.grey.shade100,
        borderRadius: BorderRadius.circular(12),
      ),
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              const Icon(Icons.poll, size: 18, color: Colors.blue),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  poll.question,
                  style: const TextStyle(
                    fontWeight: FontWeight.w600,
                    fontSize: 15,
                  ),
                ),
              ),
            ],
          ),
          if (poll.multiSelect)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                'Select multiple options',
                style: TextStyle(
                  fontSize: 12,
                  color: Colors.grey.shade600,
                ),
              ),
            ),
          const SizedBox(height: 12),
          ...poll.options.map((option) => _buildOption(context, option)),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                '${poll.totalVotes} vote${poll.totalVotes == 1 ? '' : 's'}',
                style: TextStyle(
                  fontSize: 12,
                  color: Colors.grey.shade600,
                ),
              ),
              if (poll.closed)
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: Colors.red.shade100,
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    'Closed',
                    style: TextStyle(
                      fontSize: 11,
                      color: Colors.red.shade700,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                )
              else if (isCreator && onClose != null)
                TextButton(
                  onPressed: onClose,
                  style: TextButton.styleFrom(
                    padding: const EdgeInsets.symmetric(horizontal: 8),
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  child: const Text('Close Poll', style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
          if (poll.anonymous)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Row(
                children: [
                  Icon(Icons.visibility_off, size: 12, color: Colors.grey.shade500),
                  const SizedBox(width: 4),
                  Text(
                    'Anonymous poll',
                    style: TextStyle(fontSize: 11, color: Colors.grey.shade500),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildOption(BuildContext context, PollOption option) {
    final canVote = !poll.closed;
    final hasVoted = poll.options.any((o) => o.votedByMe);

    return GestureDetector(
      onTap: canVote ? () => onVote(option.id) : null,
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        decoration: BoxDecoration(
          border: Border.all(
            color: option.votedByMe ? Colors.blue : Colors.grey.shade300,
            width: option.votedByMe ? 2 : 1,
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Stack(
          children: [
            // Progress bar
            if (hasVoted || poll.closed)
              ClipRRect(
                borderRadius: BorderRadius.circular(7),
                child: LinearProgressIndicator(
                  value: option.percentage / 100,
                  backgroundColor: Colors.transparent,
                  valueColor: AlwaysStoppedAnimation(
                    option.votedByMe ? Colors.blue.shade100 : Colors.grey.shade200,
                  ),
                  minHeight: 40,
                ),
              ),
            // Content
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              child: Row(
                children: [
                  if (poll.multiSelect)
                    Container(
                      width: 18,
                      height: 18,
                      margin: const EdgeInsets.only(right: 8),
                      decoration: BoxDecoration(
                        border: Border.all(
                          color: option.votedByMe ? Colors.blue : Colors.grey,
                        ),
                        borderRadius: BorderRadius.circular(4),
                        color: option.votedByMe ? Colors.blue : null,
                      ),
                      child: option.votedByMe
                          ? const Icon(Icons.check, size: 14, color: Colors.white)
                          : null,
                    )
                  else
                    Container(
                      width: 18,
                      height: 18,
                      margin: const EdgeInsets.only(right: 8),
                      decoration: BoxDecoration(
                        border: Border.all(
                          color: option.votedByMe ? Colors.blue : Colors.grey,
                          width: option.votedByMe ? 5 : 1,
                        ),
                        shape: BoxShape.circle,
                      ),
                    ),
                  Expanded(
                    child: Text(
                      option.text,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: option.votedByMe ? FontWeight.w600 : FontWeight.normal,
                      ),
                    ),
                  ),
                  if (hasVoted || poll.closed)
                    Text(
                      '${option.percentage.toStringAsFixed(0)}%',
                      style: TextStyle(
                        fontSize: 13,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
