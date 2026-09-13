import 'package:audio_service/audio_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/models.dart';
import '../ui/cover_image.dart';
import 'player_controller.dart';

/// The full-screen player.
///
/// Media-heavy surfaces are the documented exception to the tonal surface
/// palette: the artwork is the content, and a neutral dark ground keeps
/// attention on it (and saves power on OLED). Everything else — type scale,
/// component choice, state layers — is still straight M3.
class PlayerScreen extends ConsumerWidget {
  const PlayerScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final item = ref.watch(currentMediaItemProvider).valueOrNull;
    final state = ref.watch(playbackStateProvider).valueOrNull;

    if (item == null) {
      return Scaffold(
        appBar: AppBar(),
        body: const Center(child: Text('Nothing is playing')),
      );
    }

    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;
    final playing = state?.playing ?? false;
    final buffering = state?.processingState == AudioProcessingState.loading ||
        state?.processingState == AudioProcessingState.buffering;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Now playing'),
        actions: <Widget>[
          IconButton(
            tooltip: 'Queue',
            icon: const Icon(Icons.queue_music),
            onPressed: () => _showQueue(context, ref),
          ),
        ],
      ),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            return SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
              child: ConstrainedBox(
                constraints: BoxConstraints(minHeight: constraints.maxHeight - 32),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: <Widget>[
                    ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 420, maxHeight: 420),
                      child: AspectRatio(
                        aspectRatio: 1,
                        child: CoverImage(
                          coverUrl: item.artUri?.path,
                          borderRadius: 16,
                        ),
                      ),
                    ),
                    const SizedBox(height: 32),
                    Text(
                      item.title,
                      style: text.headlineSmall,
                      textAlign: TextAlign.center,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      item.artist ?? '',
                      style: text.titleMedium?.copyWith(color: colors.onSurfaceVariant),
                      textAlign: TextAlign.center,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 24),
                    _Scrubber(state: state, duration: item.duration),
                    const SizedBox(height: 8),
                    _Transport(playing: playing, buffering: buffering),
                    const SizedBox(height: 16),
                    _Modes(state: state),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  void _showQueue(BuildContext context, WidgetRef ref) {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (context) => const _QueueSheet(),
    );
  }
}

class _Scrubber extends ConsumerStatefulWidget {
  const _Scrubber({required this.state, required this.duration});

  final PlaybackState? state;
  final Duration? duration;

  @override
  ConsumerState<_Scrubber> createState() => _ScrubberState();
}

class _ScrubberState extends ConsumerState<_Scrubber> {
  /// Where the user has dragged to. While this is set the slider follows the
  /// finger rather than the playhead, which would otherwise fight it.
  double? _dragging;

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;

    final total = widget.duration?.inMilliseconds.toDouble() ?? 0;
    final position = widget.state?.position.inMilliseconds.toDouble() ?? 0;
    final value = (_dragging ?? position).clamp(0, total <= 0 ? 1 : total).toDouble();

    return Column(
      children: <Widget>[
        Slider(
          value: value,
          max: total <= 0 ? 1 : total,
          onChanged: total <= 0 ? null : (v) => setState(() => _dragging = v),
          onChangeEnd: total <= 0
              ? null
              : (v) {
                  ref
                      .read(playerControllerProvider)
                      .seek(Duration(milliseconds: v.round()));
                  setState(() => _dragging = null);
                },
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              Text(
                formatDuration(Duration(milliseconds: value.round())),
                style: text.labelSmall?.copyWith(color: colors.onSurfaceVariant),
              ),
              Text(
                formatDuration(widget.duration),
                style: text.labelSmall?.copyWith(color: colors.onSurfaceVariant),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _Transport extends ConsumerWidget {
  const _Transport({required this.playing, required this.buffering});

  final bool playing;
  final bool buffering;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final player = ref.read(playerControllerProvider);

    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: <Widget>[
        IconButton.filledTonal(
          tooltip: 'Previous',
          iconSize: 32,
          onPressed: player.skipToPrevious,
          icon: const Icon(Icons.skip_previous),
        ),
        const SizedBox(width: 16),
        IconButton.filled(
          tooltip: playing ? 'Pause' : 'Play',
          iconSize: 40,
          padding: const EdgeInsets.all(16),
          onPressed: playing ? player.pause : player.play,
          icon: buffering
              ? const SizedBox(
                  width: 40,
                  height: 40,
                  child: Padding(
                    padding: EdgeInsets.all(6),
                    child: CircularProgressIndicator(strokeWidth: 3),
                  ),
                )
              : Icon(playing ? Icons.pause : Icons.play_arrow),
        ),
        const SizedBox(width: 16),
        IconButton.filledTonal(
          tooltip: 'Next',
          iconSize: 32,
          onPressed: player.skipToNext,
          icon: const Icon(Icons.skip_next),
        ),
      ],
    );
  }
}

class _Modes extends ConsumerWidget {
  const _Modes({required this.state});

  final PlaybackState? state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final player = ref.read(playerControllerProvider);
    final shuffle = state?.shuffleMode == AudioServiceShuffleMode.all;
    final repeat = state?.repeatMode ?? AudioServiceRepeatMode.none;

    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: <Widget>[
        IconButton(
          tooltip: shuffle ? 'Shuffle on' : 'Shuffle off',
          isSelected: shuffle,
          selectedIcon: const Icon(Icons.shuffle_on_outlined),
          icon: const Icon(Icons.shuffle),
          onPressed: () => player.toggleShuffle(!shuffle),
        ),
        const SizedBox(width: 24),
        IconButton(
          tooltip: switch (repeat) {
            AudioServiceRepeatMode.one => 'Repeat this track',
            AudioServiceRepeatMode.none => 'Repeat off',
            _ => 'Repeat queue',
          },
          isSelected: repeat != AudioServiceRepeatMode.none,
          selectedIcon: Icon(
            repeat == AudioServiceRepeatMode.one ? Icons.repeat_one_on_outlined : Icons.repeat_on_outlined,
          ),
          icon: const Icon(Icons.repeat),
          onPressed: () => player.cycleRepeat(repeat),
        ),
      ],
    );
  }
}

class _QueueSheet extends ConsumerWidget {
  const _QueueSheet();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final queue = ref.watch(playbackQueueProvider).valueOrNull ?? const <MediaItem>[];
    final current = ref.watch(playbackStateProvider).valueOrNull?.queueIndex;
    final text = Theme.of(context).textTheme;

    return SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.fromLTRB(24, 0, 24, 8),
            child: Row(
              children: <Widget>[
                Text('Queue', style: text.titleLarge),
                const Spacer(),
                Text('${queue.length} tracks', style: text.bodySmall),
              ],
            ),
          ),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: queue.length,
              itemBuilder: (context, index) {
                final item = queue[index];
                return ListTile(
                  selected: index == current,
                  leading: index == current
                      ? const Icon(Icons.volume_up)
                      : SizedBox(
                          width: 24,
                          child: Text('${index + 1}', textAlign: TextAlign.center),
                        ),
                  title: Text(item.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                  subtitle: Text(item.artist ?? '', maxLines: 1),
                  trailing: Text(formatDuration(item.duration), style: text.labelSmall),
                  onTap: () {
                    Navigator.of(context).pop();
                    ref.read(playerControllerProvider).skipToQueueItem(index);
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
