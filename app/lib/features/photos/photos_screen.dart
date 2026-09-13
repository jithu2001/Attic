import 'package:flutter/material.dart';

import '../../core/flags.dart';
import '../../core/widgets/async_view.dart';

class PhotosScreen extends StatelessWidget {
  const PhotosScreen({super.key});

  @override
  Widget build(BuildContext context) {
    if (!Flags.photos) {
      return Scaffold(
        appBar: AppBar(title: const Text('Photos')),
        body: const EmptyState(
          icon: Icons.photo_library_outlined,
          title: 'Photos',
          message: 'Coming soon',
        ),
      );
    }
    // The real timeline is built in the photos phase.
    return Scaffold(
      appBar: AppBar(title: const Text('Photos')),
      body: const EmptyState(
        icon: Icons.photo_library_outlined,
        title: 'Photos',
        message: 'Photo library under construction.',
      ),
    );
  }
}
