import 'package:flutter/material.dart';

import '../layout/window_size.dart';

/// Shared empty state for destinations whose feature is still behind a flag.
///
/// It is a real M3 screen, not a stub: correct colour roles, type scale and
/// screen margins, so the shell is demoable from day one.
class PlaceholderScreen extends StatelessWidget {
  const PlaceholderScreen({
    super.key,
    required this.title,
    required this.icon,
    required this.message,
    this.arrivesIn,
  });

  final String title;
  final IconData icon;
  final String message;

  /// Optional note about which phase turns this destination on.
  final String? arrivesIn;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final margin = WindowSizeClass.of(context).screenMargin;

    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Center(
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: margin),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Container(
                  padding: const EdgeInsets.all(24),
                  decoration: BoxDecoration(
                    color: colors.secondaryContainer,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(icon, size: 40, color: colors.onSecondaryContainer),
                ),
                const SizedBox(height: 24),
                Text(
                  title,
                  style: text.headlineSmall,
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 8),
                Text(
                  message,
                  style: text.bodyMedium?.copyWith(color: colors.onSurfaceVariant),
                  textAlign: TextAlign.center,
                ),
                if (arrivesIn != null) ...<Widget>[
                  const SizedBox(height: 16),
                  Chip(
                    avatar: const Icon(Icons.schedule, size: 18),
                    label: Text(arrivesIn!),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
