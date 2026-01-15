import 'package:flutter/material.dart';

class PollCreateDialog extends StatefulWidget {
  final String? groupId;
  final String? recipientId;
  final Function(String question, List<String> options, bool multiSelect, bool anonymous) onSubmit;

  const PollCreateDialog({
    super.key,
    this.groupId,
    this.recipientId,
    required this.onSubmit,
  });

  @override
  State<PollCreateDialog> createState() => _PollCreateDialogState();
}

class _PollCreateDialogState extends State<PollCreateDialog> {
  final _questionController = TextEditingController();
  final List<TextEditingController> _optionControllers = [
    TextEditingController(),
    TextEditingController(),
  ];
  bool _multiSelect = false;
  bool _anonymous = false;

  @override
  void dispose() {
    _questionController.dispose();
    for (final c in _optionControllers) {
      c.dispose();
    }
    super.dispose();
  }

  void _addOption() {
    if (_optionControllers.length < 10) {
      setState(() {
        _optionControllers.add(TextEditingController());
      });
    }
  }

  void _removeOption(int index) {
    if (_optionControllers.length > 2) {
      setState(() {
        _optionControllers[index].dispose();
        _optionControllers.removeAt(index);
      });
    }
  }

  void _submit() {
    final question = _questionController.text.trim();
    if (question.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please enter a question')),
      );
      return;
    }

    final options = _optionControllers
        .map((c) => c.text.trim())
        .where((o) => o.isNotEmpty)
        .toList();

    if (options.length < 2) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please enter at least 2 options')),
      );
      return;
    }

    widget.onSubmit(question, options, _multiSelect, _anonymous);
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      child: Container(
        constraints: const BoxConstraints(maxWidth: 400, maxHeight: 600),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            AppBar(
              title: const Text('Create Poll'),
              automaticallyImplyLeading: false,
              actions: [
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed: () => Navigator.of(context).pop(),
                ),
              ],
            ),
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    TextField(
                      controller: _questionController,
                      decoration: const InputDecoration(
                        labelText: 'Question',
                        hintText: 'Ask a question...',
                        border: OutlineInputBorder(),
                      ),
                      maxLines: 2,
                    ),
                    const SizedBox(height: 16),
                    const Text(
                      'Options',
                      style: TextStyle(fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(height: 8),
                    ...List.generate(_optionControllers.length, (index) {
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Row(
                          children: [
                            Expanded(
                              child: TextField(
                                controller: _optionControllers[index],
                                decoration: InputDecoration(
                                  hintText: 'Option ${index + 1}',
                                  border: const OutlineInputBorder(),
                                  contentPadding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                    vertical: 8,
                                  ),
                                ),
                              ),
                            ),
                            if (_optionControllers.length > 2)
                              IconButton(
                                icon: const Icon(Icons.remove_circle_outline),
                                color: Colors.red,
                                onPressed: () => _removeOption(index),
                              ),
                          ],
                        ),
                      );
                    }),
                    if (_optionControllers.length < 10)
                      TextButton.icon(
                        onPressed: _addOption,
                        icon: const Icon(Icons.add),
                        label: const Text('Add Option'),
                      ),
                    const SizedBox(height: 16),
                    CheckboxListTile(
                      title: const Text('Allow multiple selections'),
                      value: _multiSelect,
                      onChanged: (v) => setState(() => _multiSelect = v ?? false),
                      contentPadding: EdgeInsets.zero,
                    ),
                    CheckboxListTile(
                      title: const Text('Anonymous voting'),
                      subtitle: const Text('Voters won\'t be shown'),
                      value: _anonymous,
                      onChanged: (v) => setState(() => _anonymous = v ?? false),
                      contentPadding: EdgeInsets.zero,
                    ),
                  ],
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: ElevatedButton(
                onPressed: _submit,
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(48),
                ),
                child: const Text('Create Poll'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
