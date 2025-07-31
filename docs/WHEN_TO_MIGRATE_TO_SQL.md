# When to Migrate from JSON to SQL

## Current JSON Approach

```
.loco/
├── project.json         # Project context
└── sessions/
    ├── chat_1234.json   # Individual sessions
    ├── chat_5678.json
    └── ...
```

## Signs It's Time to Switch

### 1. Performance Issues
- [ ] Loading session list takes > 500ms
- [ ] Searching through messages is slow
- [ ] Users have > 1000 sessions
- [ ] Session files are > 10MB each

### 2. Feature Limitations
- [ ] Need full-text search across all sessions
- [ ] Want to query: "Show me all sessions about authentication"
- [ ] Need analytics: "Which models do I use most?"
- [ ] Want to share sessions between devices
- [ ] Need concurrent access (multiple Loco instances)

### 3. Data Complexity
- [ ] Adding relationships between entities
- [ ] Need atomic transactions
- [ ] Want to track model performance metrics
- [ ] Building agent work history

### 4. User Requests
- [ ] "I wish I could search my chat history"
- [ ] "Can I export all sessions from last month?"
- [ ] "Show me all tasks that failed"
- [ ] "Which files do I edit most often?"

## Migration Triggers

**Consider migrating when ANY of these happen:**

1. **The 1K Rule**: User has 1,000+ sessions
2. **The 1S Rule**: Any common operation takes > 1 second
3. **The Search Rule**: Users ask for search 3+ times
4. **The Analytics Rule**: Need any kind of aggregation

## What We'd Gain with SQLite

```sql
-- Fast queries
SELECT * FROM messages 
WHERE content LIKE '%bug%' 
ORDER BY created_at DESC;

-- Analytics
SELECT model_name, COUNT(*) as usage_count
FROM sessions
GROUP BY model_name;

-- Full-text search
SELECT * FROM messages_fts
WHERE messages_fts MATCH 'authentication';

-- Agent performance
SELECT agent_id, task_type, 
       AVG(completion_time) as avg_time,
       SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_rate
FROM agent_tasks
GROUP BY agent_id, task_type;
```

## Migration Strategy (When the Time Comes)

1. **Keep JSON as backup** - SQLite for queries, JSON for portability
2. **Gradual migration** - New features use SQL, old ones read JSON
3. **Export/Import** - Always support JSON export for data freedom
4. **Use sqlc** - Generate Go code from SQL, type-safe queries

## The Sweet Spot

JSON is PERFECT for:
- Prototyping (where we are now)
- < 100 sessions
- Single-user, single-device
- Simple chat history

SQL becomes NECESSARY for:
- Multi-agent coordination
- Performance analytics  
- Search functionality
- Multi-device sync

## Current Status: ✅ JSON is Fine

We have:
- ~10 sessions max during testing
- No search requirements yet
- Single user, local only
- Simple linear chats

**Verdict: Stick with JSON until we hit a trigger above**