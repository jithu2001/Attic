import 'package:flutter/material.dart';

import '../../core/flags.dart';
import '../../core/widgets/placeholder_screen.dart';

class MusicScreen extends StatelessWidget {
  const MusicScreen({super.key});

  @override
  Widget build(BuildContext context) {
    if (!Flags.music) {
      return const PlaceholderScreen(
        title: 'Music',
        icon: Icons.library_music_outlined,
        message:
            'Albums, artists and playlists from your own library, with gapless '
            'playback and background audio.',
        arrivesIn: 'Coming in a later phase',
      );
    }
    return const PlaceholderScreen(
      title: 'Music',
      icon: Icons.library_music_outlined,
      message: 'Music library under construction.',
    );
  }
}
