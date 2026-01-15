import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'providers/auth_provider.dart';
import 'screens/login_screen.dart';
import 'screens/contacts_screen.dart';
import 'widgets/connection_status_banner.dart';

void main() {
  runApp(
    const ProviderScope(
      child: MessengerApp(),
    ),
  );
}

class MessengerApp extends ConsumerWidget {
  const MessengerApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return MaterialApp(
      title: 'Messenger',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: Colors.blue,
          brightness: Brightness.light,
        ),
        useMaterial3: true,
        inputDecorationTheme: InputDecorationTheme(
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      darkTheme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: Colors.blue,
          brightness: Brightness.dark,
        ),
        useMaterial3: true,
        inputDecorationTheme: InputDecorationTheme(
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      themeMode: ThemeMode.system,
      home: const AuthWrapper(),
    );
  }
}

class AuthWrapper extends ConsumerWidget {
  const AuthWrapper({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authProvider);

    Widget child;
    switch (authState.status) {
      case AuthStatus.unknown:
        child = const Scaffold(
          body: Center(
            child: CircularProgressIndicator(),
          ),
        );
        break;
      case AuthStatus.authenticated:
        child = const ContactsScreen();
        break;
      case AuthStatus.unauthenticated:
        child = const LoginScreen();
        break;
    }

    // Wrap authenticated screens with connection status banner
    if (authState.status == AuthStatus.authenticated) {
      return ConnectionAwareWrapper(child: child);
    }
    return child;
  }
}
