import 'package:flutter/material.dart';

import '../../core/flags.dart';
import '../../core/widgets/placeholder_screen.dart';

class VideoScreen extends StatelessWidget {
  const VideoScreen({super.key});

  @override
  Widget build(BuildContext context) {
    if (!Flags.video) {
      return const PlaceholderScreen(
        title: 'Video',
        icon: Icons.movie_outlined,
        message:
            'Movies and series with posters, resume-where-you-left-off and '
            'hardware-accelerated streaming to phone and TV.',
        arrivesIn: 'Coming in a later phase',
      );
    }
    return const PlaceholderScreen(
      title: 'Video',
      icon: Icons.movie_outlined,
      message: 'Video library under construction.',
    );
  }
}
