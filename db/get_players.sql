DELIMITER //
USE ih_login_server//
DROP PROCEDURE IF EXISTS get_players//
CREATE PROCEDURE get_players()
BEGIN
    DECLARE account VARCHAR(64);
	DECLARE unique_id VARCHAR(64);
	DECLARE last_server_id INT;
	DECLARE s1_pid, s2_pid, s3_pid, s4_pid, s5_pid/*, s6_pid, s7_pid, s8_pid*/ INT DEFAULT 0;
	DECLARE s1_name, s2_name, s3_name, s4_name, s5_name/*, s6_name, s7_name, s8_name*/ VARCHAR(32) DEFAULT '';
    DECLARE done INT DEFAULT 0;

	DECLARE cur CURSOR FOR (SELECT AccountId, UniqueId, LastSelectServerId FROM Accounts GROUP BY UniqueId);
	DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;
	
	CREATE TEMPORARY TABLE IF NOT EXISTS tmp_players (
		Account CHAR(64),
 		UniqueId CHAR(64),
		S1_PID INT(11) UNSIGNED NOT NULL,
		S1_NAME CHAR(64),
		S2_PID INT(11) UNSIGNED NOT NULL,
		S2_NAME CHAR(64),
		S3_PID INT(11) UNSIGNED NOT NULL,
		S3_NAME CHAR(64),
		S4_PID INT(11) UNSIGNED NOT NULL,
		S4_NAME CHAR(64),
		S5_PID INT(11) UNSIGNED NOT NULL,
		S5_NAME CHAR(64),
		/*S6_PID INT(11) UNSIGNED NOT NULL,
		S6_NAME CHAR(64),
		S7_PID INT(11) UNSIGNED NOT NULL,
		S7_NAME CHAR(64),
		S8_PID INT(11) UNSIGNED NOT NULL,
		S8_NAME CHAR(64),*/
 		PRIMARY KEY (Account)
 	);
	
	/*SET @s1_pid=0, @s2_pid=0, @s3_pid=0, @s4_pid=0, @s5_pid=0, @s6_pid=0, @s7_pid=0, @s8_pid=0;
	SET @s1_name='', @s2_name='', @s3_name='', @s4_name='', @s5_name='', @s6_name='', @s7_name='', @s8_name=''; 
	
	SET @s1_sql = 'SELECT PlayerId, Name INTO s1_pid, s1_name FROM ih_hall_server.Players WHERE UniqueId=unique_id';
	PREPARE st_s1 FROM @s1_sql;
	SET @s2_sql = 'SELECT PlayerId, Name INTO s2_pid, s2_name FROM ih_hall_server_2.Players WHERE UniqueId=unique_id';
	PREPARE st_s2 FROM @s2_sql;
	SET @s3_sql = 'SELECT PlayerId, Name INTO s3_pid, s3_name FROM ih_hall_server_3.Players WHERE UniqueId=unique_id';
	PREPARE st_s3 FROM @s3_sql;
	SET @s4_sql = 'SELECT PlayerId, Name INTO s4_pid, s4_name FROM ih_hall_server_4.Players WHERE UniqueId=unique_id';
	PREPARE st_s4 FROM @s4_sql;
	SET @s5_sql = 'SELECT PlayerId, Name INTO s5_pid, s5_name FROM ih_hall_server_5.Players WHERE UniqueId=unique_id';
	PREPARE st_s5 FROM @s5_sql;
	SET @s6_sql = 'SELECT PlayerId, Name INTO s6_pid, s6_name FROM ih_hall_server_6.Players WHERE UniqueId=unique_id';
	PREPARE st_s6 FROM @s6_sql;
	SET @s7_sql = 'SELECT PlayerId, Name INTO s7_pid, s7_name FROM ih_hall_server_7.Players WHERE UniqueId=unique_id';
	PREPARE st_s7 FROM @s7_sql;
	SET @s8_sql = 'SELECT PlayerId, Name INTO s8_pid, s8_name FROM ih_hall_server_8.Players WHERE UniqueId=unique_id';
	PREPARE st_s8 FROM @s8_sql;*/

	OPEN cur;
	it_loop: LOOP
		FETCH cur INTO account, unique_id, last_server_id;
		IF done = 1 THEN
			LEAVE it_loop;
		END IF;
		
		/*EXECUTE st_s1;
		EXECUTE st_s2;
		EXECUTE st_s3;
		EXECUTE st_s4;
		EXECUTE st_s5;
		EXECUTE st_s6;
		EXECUTE st_s7;
		EXECUTE st_s8;*/
		SELECT PlayerId, Name INTO s1_pid, s1_name FROM ih_hall_server.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s2_pid, s2_name FROM ih_hall_server_2.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s3_pid, s3_name FROM ih_hall_server_3.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s4_pid, s4_name FROM ih_hall_server_4.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s5_pid, s5_name FROM ih_hall_server_5.Players WHERE UniqueId=unique_id;
		/*SELECT PlayerId, Name INTO s6_pid, s6_name FROM ih_hall_server_6.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s7_pid, s7_name FROM ih_hall_server_7.Players WHERE UniqueId=unique_id;
		SELECT PlayerId, Name INTO s8_pid, s8_name FROM ih_hall_server_8.Players WHERE UniqueId=unique_id;*/
		IF done = 1 THEN
			SET done = 0;
		END IF;
		
		INSERT INTO tmp_players (Account, UniqueId, S1_PID, S1_NAME, S2_PID, S2_NAME, S3_PID, S3_NAME, S4_PID, S4_NAME, S5_PID, S5_NAME/*, S6_PID, S6_NAME, S7_PID, S7_NAME, S8_PID, S8_NAME*/)
		VALUES (account, unique_id, s1_pid, s1_name, s2_pid, s2_name, s3_pid, s3_name, s4_pid, s4_name, s5_pid, s5_name/*, s6_pid, s6_name, s7_pid, s7_name, s8_pid, s8_name*/);
	END LOOP;
	CLOSE cur;
    SELECT * INTO OUTFILE '/tmp/players.xls' FROM tmp_players;
	DROP TABLE tmp_players;
END;
//
DELIMITER ;

CALL get_players();
