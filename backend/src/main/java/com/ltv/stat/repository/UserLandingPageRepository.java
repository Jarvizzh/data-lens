package com.ltv.stat.repository;

import com.ltv.stat.entity.UserLandingPage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Repository
public interface UserLandingPageRepository extends JpaRepository<UserLandingPage, Long> {
    List<UserLandingPage> findByUserId(Long userId);

    @Query("SELECT p FROM UserLandingPage p WHERE (p.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (p.platformCode IS NULL OR p.platformCode = '' OR p.platformCode = 'ALL')))")
    List<UserLandingPage> findByPlatformCode(@Param("platformCode") String platformCode);

    @Query("SELECT p FROM UserLandingPage p WHERE (p.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (p.platformCode IS NULL OR p.platformCode = '' OR p.platformCode = 'ALL'))) AND p.userId = :userId")
    List<UserLandingPage> findByPlatformCodeAndUserId(@Param("platformCode") String platformCode, @Param("userId") Long userId);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM UserLandingPage p WHERE p.userId = :userId")
    void deleteByUserId(@Param("userId") Long userId);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM UserLandingPage p WHERE (p.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (p.platformCode IS NULL OR p.platformCode = '' OR p.platformCode = 'ALL'))) AND p.userId = :userId")
    void deleteByPlatformCodeAndUserId(@Param("platformCode") String platformCode, @Param("userId") Long userId);

    boolean existsByUserIdAndLandingPageId(Long userId, String landingPageId);
    boolean existsByPlatformCodeAndUserIdAndLandingPageId(String platformCode, Long userId, String landingPageId);
}
