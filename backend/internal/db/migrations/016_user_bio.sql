-- 用户个人简介。
--
-- 用途：/api/user/info 返回 bio；/api/user/updateBasicInfo 接受 bio
-- （≤255 字，入库前 trim）。
--
-- 设计要点：varchar(255) NOT NULL DEFAULT '' 而不是 NULL —— 简介是
-- "没填"与"填了空"语义相同的一列，空串让所有读取方不必处理 NULL。
ALTER TABLE `cc_users`
  ADD COLUMN `u_bio` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '个人简介';
