-- Teacher profile media as uploaded files (the onboarding wizard uploads
-- through POST /v1/uploads instead of pasting URLs). The *_url text columns
-- stay: the seed's external placeholder photos still live there, and when an
-- asset is set the service writes the public media route path into the
-- matching *_url column so every existing reader (cards, bookings, admin)
-- keeps working unchanged. No ON DELETE: file_assets rows are never deleted.
ALTER TABLE teachers
    ADD COLUMN avatar_asset_id      uuid REFERENCES file_assets (id),
    ADD COLUMN intro_video_asset_id uuid REFERENCES file_assets (id);
