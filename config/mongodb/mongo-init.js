// MongoDB Initialization Script for Address Parser Service
// ======================================================

// Switch to address_parser database
db = db.getSiblingDB('address_parser');

// Create collections with proper indexes
print('Creating collections and indexes...');

// Address cache collection (L2 cache)
db.createCollection('address_cache');
db.address_cache.createIndex({ "hash": 1 }, { unique: true });
db.address_cache.createIndex({ "created_at": 1 }, { expireAfterSeconds: 86400 * 30 }); // 30 days TTL
db.address_cache.createIndex({ "confidence": 1 });
db.address_cache.createIndex({ "raw_address": "text" });

// Gazetteer data collection
db.createCollection('gazetteer');
db.gazetteer.createIndex({ "type": 1 });
db.gazetteer.createIndex({ "level": 1 });
db.gazetteer.createIndex({ "code": 1 }, { unique: true });
db.gazetteer.createIndex({ "name": "text" });
db.gazetteer.createIndex({ "aliases": "text" });
db.gazetteer.createIndex({ "parent_code": 1 });

// Address parsing results
db.createCollection('parsing_results');
db.parsing_results.createIndex({ "job_id": 1 });
db.parsing_results.createIndex({ "created_at": 1 });
db.parsing_results.createIndex({ "confidence": 1 });
db.parsing_results.createIndex({ "status": 1 });

// Review queue for low confidence results
db.createCollection('review_queue');
db.review_queue.createIndex({ "created_at": 1 });
db.review_queue.createIndex({ "confidence": 1 });
db.review_queue.createIndex({ "status": 1 });
db.review_queue.createIndex({ "reviewer_id": 1 });

// Performance metrics
db.createCollection('metrics');
db.metrics.createIndex({ "timestamp": 1 });
db.metrics.createIndex({ "metric_type": 1 });
db.metrics.createIndex({ "service": 1 });

// Create admin user for the database
db.createUser({
  user: 'address_parser_user',
  pwd: 'address_parser_pass',
  roles: [
    { role: 'readWrite', db: 'address_parser' },
    { role: 'dbAdmin', db: 'address_parser' }
  ]
});

// Insert initial gazetteer data structure
db.gazetteer.insertMany([
  {
    type: 'country',
    level: 1,
    code: 'VN',
    name: 'Việt Nam',
    aliases: ['Vietnam', 'Việt Nam', 'VN'],
    parent_code: null,
    created_at: new Date(),
    updated_at: new Date()
  },
  {
    type: 'province',
    level: 2,
    code: '79',
    name: 'Thành phố Hồ Chí Minh',
    aliases: ['TP.HCM', 'TPHCM', 'Ho Chi Minh City', 'Saigon'],
    parent_code: 'VN',
    created_at: new Date(),
    updated_at: new Date()
  },
  {
    type: 'district',
    level: 3,
    code: '760',
    name: 'Quận 1',
    aliases: ['District 1', 'Q1', 'Quan 1'],
    parent_code: '79',
    created_at: new Date(),
    updated_at: new Date()
  }
]);

print('Database initialization completed successfully!');
print('Collections created: address_cache, gazetteer, parsing_results, review_queue, metrics');
print('Indexes created for optimal performance');
print('Admin user created: address_parser_user');
print('Initial gazetteer data inserted');
