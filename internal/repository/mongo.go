package repository

import (
	"context"
	"matter-core/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepo struct {
	client      *mongo.Client
	db          *mongo.Database
	schemas     *mongo.Collection
	entries     *mongo.Collection
	users       *mongo.Collection
	taxonomy    *mongo.Collection
	terms       *mongo.Collection
	comments    *mongo.Collection
	sessions    *mongo.Collection
	oauthStates *mongo.Collection
}

func NewMongoRepo(uri, dbName string) (*MongoRepo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	repo := &MongoRepo{
		client:      client,
		db:          db,
		schemas:     db.Collection("schemas"),
		entries:     db.Collection("entries"),
		users:       db.Collection("users"),
		taxonomy:    db.Collection("taxonomies"),
		terms:       db.Collection("terms"),
		comments:    db.Collection("comments"),
		sessions:    db.Collection("sessions"),
		oauthStates: db.Collection("oauth_states"),
	}

	if err := repo.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *MongoRepo) ensureIndexes(ctx context.Context) error {
	// Schema indexes
	_, err := r.schemas.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "key", Value: 1}, {Key: "version", Value: -1}}},
	})
	if err != nil {
		return err
	}

	// Entry wildcard index for attributes
	_, err = r.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "attributes.$**", Value: 1}}},
		{Keys: bson.D{{Key: "schema_key", Value: 1}}},
		{Keys: bson.D{{Key: "author_id", Value: 1}}},
	})
	if err != nil {
		return err
	}

	// User indexes
	_, err = r.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "socials.provider", Value: 1}, {Key: "socials.provider_user_id", Value: 1}}},
	})
	if err != nil {
		return err
	}

	// Taxonomy indexes
	_, err = r.taxonomy.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Term indexes
	_, err = r.terms.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "taxonomy_key", Value: 1}}},
		{Keys: bson.D{{Key: "slug", Value: 1}}},
	})
	if err != nil {
		return err
	}

	// Comment indexes
	_, err = r.comments.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "entry_id", Value: 1}}},
		{Keys: bson.D{{Key: "root_id", Value: 1}}},
	})
	if err != nil {
		return err
	}

	// Session indexes
	_, err = r.sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	if err != nil {
		return err
	}

	// OAuth state indexes
	_, err = r.oauthStates.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "state", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (r *MongoRepo) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}

// --- Schema Operations ---
func (r *MongoRepo) CreateSchema(ctx context.Context, schema *model.Schema) error {
	schema.CreatedAt = time.Now()
	result, err := r.schemas.InsertOne(ctx, schema)
	if err != nil {
		return err
	}
	schema.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetLatestSchema(ctx context.Context, key string) (*model.Schema, error) {
	var schema model.Schema
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})
	err := r.schemas.FindOne(ctx, bson.M{"key": key}, opts).Decode(&schema)
	if err != nil {
		return nil, err
	}
	return &schema, nil
}

func (r *MongoRepo) GetSchemaByID(ctx context.Context, id primitive.ObjectID) (*model.Schema, error) {
	var schema model.Schema
	err := r.schemas.FindOne(ctx, bson.M{"_id": id}).Decode(&schema)
	if err != nil {
		return nil, err
	}
	return &schema, nil
}

func (r *MongoRepo) DeleteSchemasByKey(ctx context.Context, key string) error {
	_, err := r.schemas.DeleteMany(ctx, bson.M{"key": key})
	return err
}

func (r *MongoRepo) ListSchemas(ctx context.Context) ([]model.Schema, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$sort", Value: bson.D{{Key: "version", Value: -1}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$key"},
			{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
		}}},
		{{Key: "$replaceRoot", Value: bson.D{{Key: "newRoot", Value: "$doc"}}}},
	}
	cursor, err := r.schemas.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var schemas []model.Schema
	if err := cursor.All(ctx, &schemas); err != nil {
		return nil, err
	}
	return schemas, nil
}

// --- Entry Operations ---
func (r *MongoRepo) CreateEntry(ctx context.Context, entry *model.Entry) error {
	entry.Base.CreatedAt = time.Now()
	entry.Base.UpdatedAt = time.Now()
	result, err := r.entries.InsertOne(ctx, entry)
	if err != nil {
		return err
	}
	entry.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) UpdateEntry(ctx context.Context, entry *model.Entry) error {
	entry.Base.UpdatedAt = time.Now()
	_, err := r.entries.ReplaceOne(ctx, bson.M{"_id": entry.ID}, entry)
	return err
}

func (r *MongoRepo) DeleteEntry(ctx context.Context, id primitive.ObjectID) error {
	// 先删除关联的评论
	if _, err := r.comments.DeleteMany(ctx, bson.M{"entry_id": id}); err != nil {
		return err
	}
	_, err := r.entries.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *MongoRepo) GetEntryByID(ctx context.Context, id primitive.ObjectID) (*model.Entry, error) {
	var entry model.Entry
	err := r.entries.FindOne(ctx, bson.M{"_id": id}).Decode(&entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *MongoRepo) ListEntries(ctx context.Context, schemaKey string, draft *bool, limit, offset int64) ([]model.Entry, error) {
	filter := bson.M{}
	if schemaKey != "" {
		filter["schema_key"] = schemaKey
	}
	if draft != nil {
		filter["base.draft"] = *draft
	}
	opts := options.Find().SetLimit(limit).SetSkip(offset).SetSort(bson.D{{Key: "base.created_at", Value: -1}})
	cursor, err := r.entries.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var entries []model.Entry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *MongoRepo) CountEntries(ctx context.Context, schemaKey string, draft *bool) (int64, error) {
	filter := bson.M{}
	if schemaKey != "" {
		filter["schema_key"] = schemaKey
	}
	if draft != nil {
		filter["base.draft"] = *draft
	}
	return r.entries.CountDocuments(ctx, filter)
}

func (r *MongoRepo) GetEntriesByIDs(ctx context.Context, ids []primitive.ObjectID) ([]model.Entry, error) {
	cursor, err := r.entries.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var entries []model.Entry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}

	// Preserve order from input IDs (important for search relevance)
	idToEntry := make(map[primitive.ObjectID]model.Entry, len(entries))
	for _, e := range entries {
		idToEntry[e.ID] = e
	}
	ordered := make([]model.Entry, 0, len(ids))
	for _, id := range ids {
		if e, ok := idToEntry[id]; ok {
			ordered = append(ordered, e)
		}
	}
	return ordered, nil
}

// --- User Operations ---
func (r *MongoRepo) CreateUser(ctx context.Context, user *model.User) error {
	user.CreatedAt = time.Now()
	result, err := r.users.InsertOne(ctx, user)
	if err != nil {
		return err
	}
	user.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetUserByID(ctx context.Context, id primitive.ObjectID) (*model.User, error) {
	var user model.User
	err := r.users.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *MongoRepo) GetUserBySocial(ctx context.Context, provider, providerUserID string) (*model.User, error) {
	var user model.User
	filter := bson.M{
		"socials": bson.M{
			"$elemMatch": bson.M{
				"provider":         provider,
				"provider_user_id": providerUserID,
			},
		},
	}
	err := r.users.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *MongoRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.users.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *MongoRepo) AddUserSocial(ctx context.Context, userID primitive.ObjectID, social model.SocialBind) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
		"$push": bson.M{"socials": social},
	})
	return err
}

func (r *MongoRepo) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := r.users.ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
	return err
}

// --- Taxonomy Operations ---
func (r *MongoRepo) CreateTaxonomy(ctx context.Context, tax *model.Taxonomy) error {
	result, err := r.taxonomy.InsertOne(ctx, tax)
	if err != nil {
		return err
	}
	tax.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetTaxonomyByKey(ctx context.Context, key string) (*model.Taxonomy, error) {
	var tax model.Taxonomy
	err := r.taxonomy.FindOne(ctx, bson.M{"key": key}).Decode(&tax)
	if err != nil {
		return nil, err
	}
	return &tax, nil
}

func (r *MongoRepo) ListTaxonomies(ctx context.Context) ([]model.Taxonomy, error) {
	cursor, err := r.taxonomy.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var taxonomies []model.Taxonomy
	if err := cursor.All(ctx, &taxonomies); err != nil {
		return nil, err
	}
	return taxonomies, nil
}

func (r *MongoRepo) UpdateTaxonomy(ctx context.Context, tax *model.Taxonomy) error {
	_, err := r.taxonomy.ReplaceOne(ctx, bson.M{"_id": tax.ID}, tax)
	return err
}

func (r *MongoRepo) DeleteTaxonomy(ctx context.Context, key string) error {
	_, err := r.taxonomy.DeleteOne(ctx, bson.M{"key": key})
	return err
}

// --- Term Operations ---
func (r *MongoRepo) CreateTerm(ctx context.Context, term *model.Term) error {
	result, err := r.terms.InsertOne(ctx, term)
	if err != nil {
		return err
	}
	term.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetTermByID(ctx context.Context, id primitive.ObjectID) (*model.Term, error) {
	var term model.Term
	err := r.terms.FindOne(ctx, bson.M{"_id": id}).Decode(&term)
	if err != nil {
		return nil, err
	}
	return &term, nil
}

func (r *MongoRepo) GetTermsByTaxonomy(ctx context.Context, taxonomyKey string) ([]model.Term, error) {
	cursor, err := r.terms.Find(ctx, bson.M{"taxonomy_key": taxonomyKey})
	if err != nil {
		return nil, err
	}
	var terms []model.Term
	if err := cursor.All(ctx, &terms); err != nil {
		return nil, err
	}
	return terms, nil
}

func (r *MongoRepo) GetTermBySlug(ctx context.Context, taxonomyKey, slug string) (*model.Term, error) {
	var term model.Term
	err := r.terms.FindOne(ctx, bson.M{"taxonomy_key": taxonomyKey, "slug": slug}).Decode(&term)
	if err != nil {
		return nil, err
	}
	return &term, nil
}

func (r *MongoRepo) UpdateTerm(ctx context.Context, term *model.Term) error {
	_, err := r.terms.ReplaceOne(ctx, bson.M{"_id": term.ID}, term)
	return err
}

func (r *MongoRepo) DeleteTerm(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.terms.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *MongoRepo) HasChildTerms(ctx context.Context, parentID primitive.ObjectID) (bool, error) {
	count, err := r.terms.CountDocuments(ctx, bson.M{"parent_id": parentID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *MongoRepo) HasTermReferences(ctx context.Context, taxonomyKey string, termID primitive.ObjectID) (bool, error) {
	// Check if any entry's attributes contain this term ID
	// This searches in attributes where taxonomy fields store term IDs
	termIDStr := termID.Hex()
	filter := bson.M{
		"$or": []bson.M{
			{"attributes." + taxonomyKey: termIDStr},
			{"attributes." + taxonomyKey: bson.M{"$in": []string{termIDStr}}},
		},
	}
	count, err := r.entries.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *MongoRepo) DeleteTermsByTaxonomy(ctx context.Context, taxonomyKey string) error {
	_, err := r.terms.DeleteMany(ctx, bson.M{"taxonomy_key": taxonomyKey})
	return err
}

// --- Comment Operations ---
func (r *MongoRepo) CreateComment(ctx context.Context, comment *model.Comment) error {
	comment.CreatedAt = time.Now()
	result, err := r.comments.InsertOne(ctx, comment)
	if err != nil {
		return err
	}
	comment.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetCommentByID(ctx context.Context, id primitive.ObjectID) (*model.Comment, error) {
	var comment model.Comment
	err := r.comments.FindOne(ctx, bson.M{"_id": id}).Decode(&comment)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *MongoRepo) GetCommentsByEntry(ctx context.Context, entryID primitive.ObjectID) ([]model.Comment, error) {
	cursor, err := r.comments.Find(ctx, bson.M{"entry_id": entryID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var comments []model.Comment
	if err := cursor.All(ctx, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *MongoRepo) GetCommentsByEntryPaginated(ctx context.Context, entryID primitive.ObjectID, limit, offset int64) ([]model.CommentWithAuthor, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"entry_id": entryID}}},
		{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: 1}}}},
		{{Key: "$skip", Value: offset}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "users"},
			{Key: "let", Value: bson.D{{Key: "authorId", Value: bson.D{{Key: "$toObjectId", Value: "$author_id"}}}}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$eq", Value: bson.A{"$_id", "$$authorId"}}}}}}},
				{{Key: "$project", Value: bson.D{
					{Key: "_id", Value: 1},
					{Key: "nickname", Value: 1},
					{Key: "avatar", Value: 1},
				}}},
			}},
			{Key: "as", Value: "author"},
		}}},
		{{Key: "$unwind", Value: bson.D{
			{Key: "path", Value: "$author"},
			{Key: "preserveNullAndEmptyArrays", Value: true},
		}}},
	}

	cursor, err := r.comments.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var comments []model.CommentWithAuthor
	if err := cursor.All(ctx, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *MongoRepo) CountCommentsByEntry(ctx context.Context, entryID primitive.ObjectID) (int64, error) {
	return r.comments.CountDocuments(ctx, bson.M{"entry_id": entryID})
}

func (r *MongoRepo) DeleteComment(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.comments.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *MongoRepo) IsTermSlugExists(ctx context.Context, taxonomyKey, slug string, excludeID primitive.ObjectID) (bool, error) {
	filter := bson.M{"taxonomy_key": taxonomyKey, "slug": slug}
	if !excludeID.IsZero() {
		filter["_id"] = bson.M{"$ne": excludeID}
	}
	count, err := r.terms.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *MongoRepo) UpdateComment(ctx context.Context, comment *model.Comment) error {
	comment.UpdatedAt = time.Now()
	_, err := r.comments.ReplaceOne(ctx, bson.M{"_id": comment.ID}, comment)
	return err
}

func (r *MongoRepo) DeleteCommentsByRootID(ctx context.Context, rootID primitive.ObjectID) error {
	_, err := r.comments.DeleteMany(ctx, bson.M{"root_id": rootID})
	return err
}

// --- User Update ---
func (r *MongoRepo) UpdateUserProfile(ctx context.Context, userID primitive.ObjectID, nickname, avatar string) error {
	update := bson.M{"$set": bson.M{}}
	if nickname != "" {
		update["$set"].(bson.M)["nickname"] = nickname
	}
	if avatar != "" {
		update["$set"].(bson.M)["avatar"] = avatar
	}
	if len(update["$set"].(bson.M)) == 0 {
		return nil
	}
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": userID}, update)
	return err
}

// --- Session Operations ---
func (r *MongoRepo) CreateSession(ctx context.Context, session *model.Session) error {
	session.CreatedAt = time.Now()
	result, err := r.sessions.InsertOne(ctx, session)
	if err != nil {
		return err
	}
	session.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetSessionByToken(ctx context.Context, token string) (*model.Session, error) {
	var session model.Session
	err := r.sessions.FindOne(ctx, bson.M{
		"token":      token,
		"expires_at": bson.M{"$gt": time.Now()},
	}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *MongoRepo) DeleteSession(ctx context.Context, token string) error {
	_, err := r.sessions.DeleteOne(ctx, bson.M{"token": token})
	return err
}

func (r *MongoRepo) DeleteExpiredSessions(ctx context.Context) error {
	_, err := r.sessions.DeleteMany(ctx, bson.M{"expires_at": bson.M{"$lt": time.Now()}})
	return err
}

// --- OAuth State Operations ---
func (r *MongoRepo) CreateOAuthState(ctx context.Context, state *model.OAuthState) error {
	state.CreatedAt = time.Now()
	result, err := r.oauthStates.InsertOne(ctx, state)
	if err != nil {
		return err
	}
	state.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MongoRepo) GetAndDeleteOAuthState(ctx context.Context, state string) (*model.OAuthState, error) {
	var oauthState model.OAuthState
	err := r.oauthStates.FindOneAndDelete(ctx, bson.M{"state": state}).Decode(&oauthState)
	if err != nil {
		return nil, err
	}
	return &oauthState, nil
}
