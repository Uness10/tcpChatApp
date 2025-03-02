## Yes, it is logical for a user to be automatically removed from a room when logging out. Here's why:

### Why It Makes Sense:
- Session Consistency:

    - When a user logs out, their session ends, and they should no longer participate in any active rooms.

    - Leaving them in a room after logout would create inconsistencies (e.g., they could still appear as a member but cannot send/receive messages).

- Resource Cleanup:

    - Removing the user from the room ensures that resources (e.g., memory, connection slots) are freed up.

    - This prevents "ghost users" who appear to be in a room but are no longer active.

- User Experience:

    - Logging out implies the user wants to end their current session. Being removed from the room aligns with this expectation.

If the user wants to rejoin later, they can log in again and re-enter the room.