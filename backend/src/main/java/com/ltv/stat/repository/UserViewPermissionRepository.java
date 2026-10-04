package com.ltv.stat.repository;

import com.ltv.stat.entity.UserViewPermission;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Repository
public interface UserViewPermissionRepository extends JpaRepository<UserViewPermission, Long> {

    List<UserViewPermission> findByUserId(Long userId);

    boolean existsByUserIdAndTargetUserId(Long userId, Long targetUserId);

    @Modifying
    @Transactional
    @Query("DELETE FROM UserViewPermission u WHERE u.userId = :userId")
    void deleteByUserId(@Param("userId") Long userId);

    @Modifying
    @Transactional
    @Query("DELETE FROM UserViewPermission u WHERE u.targetUserId = :targetUserId")
    void deleteByTargetUserId(@Param("targetUserId") Long targetUserId);
}
