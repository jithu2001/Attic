import 'package:flutter/material.dart';

/// A top-level destination in the app shell.
///
/// The same list feeds the compact `NavigationBar` and the expanded
/// `NavigationRail`, so the two can never drift apart.
class AppDestination {
  const AppDestination({
    required this.label,
    required this.icon,
    required this.selectedIcon,
    required this.route,
  });

  final String label;
  final IconData icon;
  final IconData selectedIcon;
  final String route;
}

const List<AppDestination> kDestinations = <AppDestination>[
  AppDestination(
    label: 'Photos',
    icon: Icons.photo_library_outlined,
    selectedIcon: Icons.photo_library,
    route: '/photos',
  ),
  AppDestination(
    label: 'Music',
    icon: Icons.library_music_outlined,
    selectedIcon: Icons.library_music,
    route: '/music',
  ),
  AppDestination(
    label: 'Video',
    icon: Icons.movie_outlined,
    selectedIcon: Icons.movie,
    route: '/video',
  ),
  AppDestination(
    label: 'Settings',
    icon: Icons.settings_outlined,
    selectedIcon: Icons.settings,
    route: '/settings',
  ),
];
