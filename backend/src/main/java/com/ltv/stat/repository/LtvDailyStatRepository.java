package com.ltv.stat.repository;

import com.ltv.stat.entity.LtvDailyStat;
import com.ltv.stat.entity.LtvDailyStatId;
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
public interface LtvDailyStatRepository extends JpaRepository<LtvDailyStat, LtvDailyStatId> {
    Optional<LtvDailyStat> findByUserIdAndLaunchDate(Long userId, LocalDate launchDate);
    Optional<LtvDailyStat> findByPlatformCodeAndUserIdAndLaunchDate(String platformCode, Long userId, LocalDate launchDate);

    List<LtvDailyStat> findByUserIdAndLaunchDateGreaterThanEqualOrderByLaunchDateAsc(Long userId, LocalDate startDate);
    List<LtvDailyStat> findByPlatformCodeAndUserIdAndLaunchDateGreaterThanEqualOrderByLaunchDateAsc(String platformCode, Long userId, LocalDate startDate);

    List<LtvDailyStat> findByUserIdOrderByLaunchDateAsc(Long userId);
    List<LtvDailyStat> findByPlatformCodeAndUserIdOrderByLaunchDateAsc(String platformCode, Long userId);

    List<LtvDailyStat> findAllByOrderByLaunchDateAsc();
    List<LtvDailyStat> findByPlatformCodeOrderByLaunchDateAsc(String platformCode);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM LtvDailyStat s WHERE s.userId = :userId")
    void deleteByUserId(@Param("userId") Long userId);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM LtvDailyStat s WHERE s.platformCode = :platformCode AND s.userId = :userId")
    void deleteByPlatformCodeAndUserId(@Param("platformCode") String platformCode, @Param("userId") Long userId);
}
