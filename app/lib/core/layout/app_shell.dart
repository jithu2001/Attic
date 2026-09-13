import 'package:flutter/material.dart';

import '../../features/music/player/mini_player.dart';
import 'destinations.dart';
import 'window_size.dart';

/// The adaptive shell around the four top-level destinations.
///
/// Compact windows (phones) get an M3 `NavigationBar` at the bottom; medium and
/// expanded windows (tablets in landscape, Android TV) get a `NavigationRail`
/// on the leading edge, which is also the D-pad-friendly arrangement on TV.
class AppShell extends StatelessWidget {
  const AppShell({
    super.key,
    required this.child,
    required this.selectedIndex,
    required this.onDestinationSelected,
  });

  final Widget child;
  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;

  @override
  Widget build(BuildContext context) {
    final sizeClass = WindowSizeClass.of(context);

    if (sizeClass.isCompact) {
      return Scaffold(
        body: SafeArea(child: child),
        // The mini-player is docked between the content and the navigation
        // bar, so it survives switching destinations and never covers one.
        bottomNavigationBar: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            const MiniPlayer(),
            NavigationBar(
              selectedIndex: selectedIndex,
              onDestinationSelected: onDestinationSelected,
              destinations: <Widget>[
                for (final d in kDestinations)
                  NavigationDestination(
                    icon: Icon(d.icon),
                    selectedIcon: Icon(d.selectedIcon),
                    label: d.label,
                    tooltip: d.label,
                  ),
              ],
            ),
          ],
        ),
      );
    }

    return Scaffold(
      body: SafeArea(
        child: Row(
          children: <Widget>[
            NavigationRail(
              selectedIndex: selectedIndex,
              onDestinationSelected: onDestinationSelected,
              // Expanded windows have room for labels; medium ones do not.
              labelType: sizeClass == WindowSizeClass.expanded
                  ? NavigationRailLabelType.none
                  : NavigationRailLabelType.selected,
              extended: sizeClass == WindowSizeClass.expanded,
              destinations: <NavigationRailDestination>[
                for (final d in kDestinations)
                  NavigationRailDestination(
                    icon: Icon(d.icon),
                    selectedIcon: Icon(d.selectedIcon),
                    label: Text(d.label),
                  ),
              ],
            ),
            const VerticalDivider(width: 1, thickness: 1),
            Expanded(
              child: Column(
                children: <Widget>[
                  Expanded(child: child),
                  const MiniPlayer(),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
