-- v32 (compatible with v10+): Add room column for tags
ALTER TABLE room ADD COLUMN tags TEXT;
UPDATE room
SET tags = (
	SELECT rad.content
	FROM room_account_data rad
	WHERE rad.room_id = room.room_id
	  AND rad.type = 'm.tag'
	  AND json_valid(rad.content)
	LIMIT 1
);
