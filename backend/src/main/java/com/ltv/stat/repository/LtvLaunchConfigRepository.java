package com.ltv.stat.repository;

import com.ltv.stat.entity.LtvLaunchConfig;
import com.ltv.stat.entity.LtvLaunchConfigId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

@Repository
public interface LtvLaunchConfigRepository extends JpaRepository<LtvLaunchConfig, LtvLaunchConfigId> {
    Optional<LtvLaunchConfig> findByUserIdAndLaunchDate(Long userId, LocalDate launchDate);

    @Query("SELECT c FROM LtvLaunchConfig c WHERE (c.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (c.platformCode IS NULL OR c.platformCode = '' OR c.platformCode = 'ALL'))) AND c.userId = :userId AND c.launchDate = :launchDate")
    Optional<LtvLaunchConfig> findByPlatformCodeAndUserIdAndLaunchDate(@Param("platformCode") String platformCode, @Param("userId") Long userId, @Param("launchDate") LocalDate launchDate);

    List<LtvLaunchConfig> findByUserId(Long userId);

    @Query("SELECT c FROM LtvLaunchConfig c WHERE (c.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (c.platformCode IS NULL OR c.platformCode = '' OR c.platformCode = 'ALL'))) AND c.userId = :userId")
    List<LtvLaunchConfig> findByPlatformCodeAndUserId(@Param("platformCode") String platformCode, @Param("userId") Long userId);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM LtvLaunchConfig c WHERE c.userId = :userId")
    void deleteByUserId(@Param("userId") Long userId);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM LtvLaunchConfig c WHERE c.platformCode = :platformCode AND c.userId = :userId")
    void deleteByPlatformCodeAndUserId(@Param("platformCode") String platformCode, @Param("userId") Long userId);
}
