import 'package:flutter/material.dart';

import '../../core/flags.dart';
import '../../core/widgets/async_view.dart';

class VideoScreen extends StatelessWidget {
  const VideoScreen({super.key});

  @override
  Widget build(BuildContext context) {
    if (!Flags.video) {
      return Scaffold(
        appBar: AppBar(title: const Text('Video')),
        body: const EmptyState(
          icon: Icons.movie_outlined,
          title: 'Video',
          message: 'Coming soon',
        ),
      );
    }
    return Scaffold(
      appBar: AppBar(title: const Text('Video')),
      body: const EmptyState(
        icon: Icons.movie_outlined,
        title: 'Video',
        message: 'Video library under construction.',
      ),
    );
  }
}
