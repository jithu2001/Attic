import 'package:audio_service/audio_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../ui/cover_image.dart';
import 'player_controller.dart';

/// The bar docked above the navigation bar whenever something is playing.
///
/// It renders nothing at all when the queue is empty, so the shell does not
/// reserve space for a player that does not exist.
class MiniPlayer extends ConsumerWidget {
  const MiniPlayer({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final item = ref.watch(currentMediaItemProvider).valueOrNull;
    if (item == null) return const SizedBox.shrink();

    final state = ref.watch(playbackStateProvider).valueOrNull;
    final playing = state?.playing ?? false;
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;

    return Material(
      color: colors.surfaceContainerHighest,
      child: InkWell(
        onTap: () => context.go('/music/player'),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            _ProgressBar(state: state, duration: item.duration),
            Padding(
              padding: const EdgeInsets.fromLTRB(8, 8, 4, 8),
              child: Row(
                children: <Widget>[
                  CoverImage(
                    coverUrl: _coverPath(item),
                    size: 40,
                    borderRadius: 8,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: <Widget>[
                        Text(
                          item.title,
                          style: text.titleSmall,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        Text(
                          item.artist ?? '',
                          style: text.bodySmall
                              ?.copyWith(color: colors.onSurfaceVariant),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    tooltip: playing ? 'Pause' : 'Play',
                    icon: Icon(playing ? Icons.pause : Icons.play_arrow),
                    onPressed: () {
                      final player = ref.read(playerControllerProvider);
                      playing ? player.pause() : player.play();
                    },
                  ),
                  IconButton(
                    tooltip: 'Next',
                    icon: const Icon(Icons.skip_next),
                    onPressed: () => ref.read(playerControllerProvider).skipToNext(),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// The queue stores fully signed URLs; the cover widget wants the relative
  /// path so its cache key stays stable as tokens rotate.
  static String? _coverPath(MediaItem item) {
    final uri = item.artUri;
    if (uri == null) return null;
    return uri.path;
  }
}

/// A hairline of progress along the top of the bar.
class _ProgressBar extends StatelessWidget {
  const _ProgressBar({required this.state, required this.duration});

  final PlaybackState? state;
  final Duration? duration;

  @override
  Widget build(BuildContext context) {
    if (state == null || duration == null || duration!.inMilliseconds == 0) {
      return const SizedBox(height: 2);
    }
    final progress =
        state!.position.inMilliseconds / duration!.inMilliseconds;
    return LinearProgressIndicator(
      value: progress.clamp(0.0, 1.0),
      minHeight: 2,
    );
  }
}
