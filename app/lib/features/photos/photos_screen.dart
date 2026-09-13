import 'package:flutter/material.dart';

import '../../core/flags.dart';
import '../../core/widgets/placeholder_screen.dart';

class PhotosScreen extends StatelessWidget {
  const PhotosScreen({super.key});

  @override
  Widget build(BuildContext context) {
    if (!Flags.photos) {
      return const PlaceholderScreen(
        title: 'Photos',
        icon: Icons.photo_library_outlined,
        message:
            'Your camera roll will back up here, and the whole library will be '
            'browsable by day, album and place.',
        arrivesIn: 'Coming in a later phase',
      );
    }
    // The real timeline is built in the photos phase.
    return const PlaceholderScreen(
      title: 'Photos',
      icon: Icons.photo_library_outlined,
      message: 'Photo library under construction.',
    );
  }
}
